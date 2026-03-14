package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/commands"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/identity"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/sipeed/picoclaw/pkg/utils"
)

var (
	reHeading        = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	reBlockquote     = regexp.MustCompile(`(?m)^>\s*(.*)$`)
	reLink           = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBoldItalicStar = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	reBoldStar       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldUnder      = regexp.MustCompile(`__(.+?)__`)
	reItalic         = regexp.MustCompile(`_([^_]+)_`)
	reStrike         = regexp.MustCompile(`~~(.+?)~~`)
	reListItem       = regexp.MustCompile(`(?m)^[-*]\s+`)
	reCodeBlock      = regexp.MustCompile("```[\\w]*\\n?([\\s\\S]*?)```")
	reInlineCode     = regexp.MustCompile("`([^`]+)`")
)

type TelegramChannel struct {
	*channels.BaseChannel
	bot     *telego.Bot
	bh      *th.BotHandler
	config  *config.Config
	chatIDs map[string]int64
	ctx     context.Context
	cancel  context.CancelFunc

	mediaMu             sync.Mutex
	lastMediaRefByChat  map[string]string
	lastMediaNameByChat map[string]string
	lastMediaSeenByChat map[string]time.Time

	registerFunc     func(context.Context, []commands.Definition) error
	commandRegCancel context.CancelFunc
}

func NewTelegramChannel(cfg *config.Config, bus *bus.MessageBus) (*TelegramChannel, error) {
	var opts []telego.BotOption
	telegramCfg := cfg.Channels.Telegram

	if telegramCfg.Proxy != "" {
		proxyURL, parseErr := url.Parse(telegramCfg.Proxy)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", telegramCfg.Proxy, parseErr)
		}
		opts = append(opts, telego.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}))
	} else if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" {
		// Use environment proxy if configured
		opts = append(opts, telego.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}))
	}

	if baseURL := strings.TrimRight(strings.TrimSpace(telegramCfg.BaseURL), "/"); baseURL != "" {
		opts = append(opts, telego.WithAPIServer(baseURL))
	}

	bot, err := telego.NewBot(telegramCfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	base := channels.NewBaseChannel(
		"telegram",
		telegramCfg,
		bus,
		telegramCfg.AllowFrom,
		channels.WithMaxMessageLength(4000),
		channels.WithGroupTrigger(telegramCfg.GroupTrigger),
		channels.WithReasoningChannelID(telegramCfg.ReasoningChannelID),
	)

	return &TelegramChannel{
		BaseChannel:         base,
		bot:                 bot,
		config:              cfg,
		chatIDs:             make(map[string]int64),
		lastMediaRefByChat:  make(map[string]string),
		lastMediaNameByChat: make(map[string]string),
		lastMediaSeenByChat: make(map[string]time.Time),
	}, nil
}

func (c *TelegramChannel) Start(ctx context.Context) error {
	logger.InfoC("telegram", "Starting Telegram bot (polling mode)...")

	c.ctx, c.cancel = context.WithCancel(ctx)

	updates, err := c.bot.UpdatesViaLongPolling(c.ctx, &telego.GetUpdatesParams{
		Timeout: 30,
	})
	if err != nil {
		c.cancel()
		return fmt.Errorf("failed to start long polling: %w", err)
	}

	bh, err := th.NewBotHandler(c.bot, updates)
	if err != nil {
		c.cancel()
		return fmt.Errorf("failed to create bot handler: %w", err)
	}
	c.bh = bh

	bh.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return c.handleMessage(ctx, &message)
	}, th.AnyMessage())

	c.SetRunning(true)
	logger.InfoCF("telegram", "Telegram bot connected", map[string]any{
		"username": c.bot.Username(),
	})

	c.startCommandRegistration(c.ctx, commands.BuiltinDefinitions())

	go func() {
		if err = bh.Start(); err != nil {
			logger.ErrorCF("telegram", "Bot handler failed", map[string]any{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

func (c *TelegramChannel) Stop(ctx context.Context) error {
	logger.InfoC("telegram", "Stopping Telegram bot...")
	c.SetRunning(false)

	// Stop the bot handler
	if c.bh != nil {
		_ = c.bh.StopWithContext(ctx)
	}

	// Cancel our context (stops long polling)
	if c.cancel != nil {
		c.cancel()
	}
	if c.commandRegCancel != nil {
		c.commandRegCancel()
	}

	return nil
}

func (c *TelegramChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID %s: %w", msg.ChatID, channels.ErrSendFailed)
	}

	if msg.Content == "" {
		return nil
	}

	// The Manager already splits messages to ≤4000 chars (WithMaxMessageLength),
	// so msg.Content is guaranteed to be within that limit. We still need to
	// check if HTML expansion pushes it beyond Telegram's 4096-char API limit.
	queue := []string{msg.Content}
	for len(queue) > 0 {
		chunk := queue[0]
		queue = queue[1:]

		logger.InfoCF("telegram", "Outbound text", map[string]any{
			"chat_id":     msg.ChatID,
			"content_len": len(chunk),
			"preview":     utils.Truncate(chunk, 160),
		})

		htmlContent := markdownToTelegramHTML(chunk)

		if len([]rune(htmlContent)) > 4096 {
			ratio := float64(len([]rune(chunk))) / float64(len([]rune(htmlContent)))
			smallerLen := int(float64(4096) * ratio * 0.95) // 5% safety margin
			if smallerLen < 100 {
				smallerLen = 100
			}
			// Push sub-chunks back to the front of the queue for
			// re-validation instead of sending them blindly.
			subChunks := channels.SplitMessage(chunk, smallerLen)
			queue = append(subChunks, queue...)
			continue
		}

		if err := c.sendHTMLChunk(ctx, chatID, htmlContent, chunk); err != nil {
			return err
		}
	}

	return nil
}

// sendHTMLChunk sends a single HTML message, falling back to the original
// markdown as plain text on parse failure so users never see raw HTML tags.
func (c *TelegramChannel) sendHTMLChunk(ctx context.Context, chatID int64, htmlContent, mdFallback string) error {
	tgMsg := tu.Message(tu.ID(chatID), htmlContent)
	tgMsg.ParseMode = telego.ModeHTML

	if _, err := c.bot.SendMessage(ctx, tgMsg); err != nil {
		logger.ErrorCF("telegram", "HTML parse failed, falling back to plain text", map[string]any{
			"error": err.Error(),
		})
		tgMsg.Text = mdFallback
		tgMsg.ParseMode = ""
		if _, err = c.bot.SendMessage(ctx, tgMsg); err != nil {
			return fmt.Errorf("telegram send: %w", channels.ErrTemporary)
		}
	}
	return nil
}

// StartTyping implements channels.TypingCapable.
// It sends ChatAction(typing) immediately and then repeats every 4 seconds
// (Telegram's typing indicator expires after ~5s) in a background goroutine.
// The returned stop function is idempotent and cancels the goroutine.
func (c *TelegramChannel) StartTyping(ctx context.Context, chatID string) (func(), error) {
	// Respect channel-level typing config; disabled means no-op.
	if !c.config.Channels.Telegram.Typing.Enabled {
		return func() {}, nil
	}

	cid, err := parseChatID(chatID)
	if err != nil {
		return func() {}, err
	}

	// Send the first typing action immediately
	_ = c.bot.SendChatAction(ctx, tu.ChatAction(tu.ID(cid), telego.ChatActionTyping))

	typingCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				_ = c.bot.SendChatAction(typingCtx, tu.ChatAction(tu.ID(cid), telego.ChatActionTyping))
			}
		}
	}()

	return cancel, nil
}

// EditMessage implements channels.MessageEditor.
func (c *TelegramChannel) EditMessage(ctx context.Context, chatID string, messageID string, content string) error {
	cid, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	mid, err := strconv.Atoi(messageID)
	if err != nil {
		return err
	}
	htmlContent := markdownToTelegramHTML(content)
	editMsg := tu.EditMessageText(tu.ID(cid), mid, htmlContent)
	editMsg.ParseMode = telego.ModeHTML
	_, err = c.bot.EditMessageText(ctx, editMsg)
	return err
}

// SendPlaceholder implements channels.PlaceholderCapable.
// It sends a placeholder message (e.g. "Thinking... 💭") that will later be
// edited to the actual response via EditMessage (channels.MessageEditor).
func (c *TelegramChannel) SendPlaceholder(ctx context.Context, chatID string) (string, error) {
	phCfg := c.config.Channels.Telegram.Placeholder
	if !phCfg.Enabled {
		return "", nil
	}

	text := phCfg.Text
	if text == "" {
		text = "Thinking... 💭"
	}

	cid, err := parseChatID(chatID)
	if err != nil {
		return "", err
	}

	pMsg, err := c.bot.SendMessage(ctx, tu.Message(tu.ID(cid), text))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", pMsg.MessageID), nil
}

// SendMedia implements the channels.MediaSender interface.
func (c *TelegramChannel) SendMedia(ctx context.Context, msg bus.OutboundMediaMessage) error {
	if !c.IsRunning() {
		return channels.ErrNotRunning
	}

	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID %s: %w", msg.ChatID, channels.ErrSendFailed)
	}

	store := c.GetMediaStore()
	if store == nil {
		return fmt.Errorf("no media store available: %w", channels.ErrSendFailed)
	}

	for _, part := range msg.Parts {
		localPath, err := store.Resolve(part.Ref)
		if err != nil {
			logger.ErrorCF("telegram", "Failed to resolve media ref", map[string]any{
				"ref":   part.Ref,
				"error": err.Error(),
			})
			continue
		}

		logger.InfoCF("telegram", "Outbound media", map[string]any{
			"chat_id":    msg.ChatID,
			"type":       part.Type,
			"caption":    utils.Truncate(part.Caption, 160),
			"local_path": localPath,
		})

		file, err := os.Open(localPath)
		if err != nil {
			logger.ErrorCF("telegram", "Failed to open media file", map[string]any{
				"path":  localPath,
				"error": err.Error(),
			})
			continue
		}

		switch part.Type {
		case "image":
			params := &telego.SendPhotoParams{
				ChatID:  tu.ID(chatID),
				Photo:   telego.InputFile{File: file},
				Caption: part.Caption,
			}
			_, err = c.bot.SendPhoto(ctx, params)
		case "audio":
			params := &telego.SendAudioParams{
				ChatID:  tu.ID(chatID),
				Audio:   telego.InputFile{File: file},
				Caption: part.Caption,
			}
			_, err = c.bot.SendAudio(ctx, params)
		case "voice":
			params := &telego.SendVoiceParams{
				ChatID:  tu.ID(chatID),
				Voice:   telego.InputFile{File: file},
				Caption: part.Caption,
			}
			_, err = c.bot.SendVoice(ctx, params)
		case "video":
			params := &telego.SendVideoParams{
				ChatID:  tu.ID(chatID),
				Video:   telego.InputFile{File: file},
				Caption: part.Caption,
			}
			_, err = c.bot.SendVideo(ctx, params)
		default: // "file" or unknown types
			params := &telego.SendDocumentParams{
				ChatID:   tu.ID(chatID),
				Document: telego.InputFile{File: file},
				Caption:  part.Caption,
			}
			_, err = c.bot.SendDocument(ctx, params)
		}

		file.Close()

		if err != nil {
			logger.ErrorCF("telegram", "Failed to send media", map[string]any{
				"type":  part.Type,
				"error": err.Error(),
			})
			return fmt.Errorf("telegram send media: %w", channels.ErrTemporary)
		}
	}

	return nil
}

func (c *TelegramChannel) handleMessage(ctx context.Context, message *telego.Message) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	user := message.From
	if user == nil {
		return fmt.Errorf("message sender (user) is nil")
	}

	platformID := fmt.Sprintf("%d", user.ID)
	sender := bus.SenderInfo{
		Platform:    "telegram",
		PlatformID:  platformID,
		CanonicalID: identity.BuildCanonicalID("telegram", platformID),
		Username:    user.Username,
		DisplayName: user.FirstName,
	}

	// check allowlist to avoid downloading attachments for rejected users
	if !c.IsAllowedSender(sender) {
		logger.DebugCF("telegram", "Message rejected by allowlist", map[string]any{
			"user_id": platformID,
		})
		return nil
	}

	chatID := message.Chat.ID
	c.chatIDs[platformID] = chatID

	content := ""
	mediaPaths := []string{}

	chatIDStr := fmt.Sprintf("%d", chatID)
	messageIDStr := fmt.Sprintf("%d", message.MessageID)
	scope := channels.BuildMediaScope("telegram", chatIDStr, messageIDStr)

	// Helper to register a local file with the media store
	storeMedia := func(localPath string, meta media.MediaMeta) string {
		if strings.TrimSpace(meta.Filename) == "" {
			meta.Filename = filepath.Base(localPath)
		}
		if strings.TrimSpace(meta.Source) == "" {
			meta.Source = "telegram"
		}

		if store := c.GetMediaStore(); store != nil {
			ref, err := store.Store(localPath, meta, scope)
			if err == nil {
				return ref
			}
			logger.WarnCF("telegram", "Failed to store media in media store", map[string]any{"error": err.Error()})
		}
		return localPath // fallback: use raw path
	}

	lastMediaName := ""

	if message.Text != "" {
		content += message.Text
	}

	if message.Caption != "" {
		if content != "" {
			content += "\n"
		}
		content += message.Caption
	}

	if len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		photoPath := c.downloadPhoto(ctx, photo.FileID)
		if photoPath != "" {
			mediaPaths = append(mediaPaths, storeMedia(photoPath, media.MediaMeta{Filename: "photo.jpg", ContentType: "image/jpeg", Source: "telegram"}))
			lastMediaName = "photo.jpg"
			if content != "" {
				content += "\n"
			}
			content += "[image: photo]"
		}
	}

	if message.Voice != nil {
		voicePath := c.downloadFile(ctx, message.Voice.FileID, ".ogg")
		if voicePath != "" {
			mediaPaths = append(mediaPaths, storeMedia(voicePath, media.MediaMeta{Filename: "voice.ogg", ContentType: "audio/ogg", Source: "telegram"}))
			lastMediaName = "voice.ogg"

			if content != "" {
				content += "\n"
			}
			content += "[voice]"
		}
	}

	if message.Audio != nil {
		audioPath := c.downloadFile(ctx, message.Audio.FileID, ".mp3")
		if audioPath != "" {
			mediaPaths = append(mediaPaths, storeMedia(audioPath, media.MediaMeta{Filename: "audio.mp3", ContentType: "audio/mpeg", Source: "telegram"}))
			lastMediaName = "audio.mp3"
			if content != "" {
				content += "\n"
			}
			content += "[audio]"
		}
	}

	if message.Video != nil {
		videoType := strings.TrimSpace(message.Video.MimeType)
		videoExt := ".mp4"
		if strings.Contains(strings.ToLower(videoType), "webm") {
			videoExt = ".webm"
		}
		videoPath := c.downloadFile(ctx, message.Video.FileID, videoExt)
		if videoPath != "" {
			if videoType == "" {
				videoType = "video/mp4"
			}
			videoName := "video" + videoExt
			mediaPaths = append(mediaPaths, storeMedia(videoPath, media.MediaMeta{Filename: videoName, ContentType: videoType, Source: "telegram"}))
			lastMediaName = videoName
			if content != "" {
				content += "\n"
			}
			content += "[video]"
		}
	}

	if message.VideoNote != nil {
		videoNotePath := c.downloadFile(ctx, message.VideoNote.FileID, ".mp4")
		if videoNotePath != "" {
			mediaPaths = append(mediaPaths, storeMedia(videoNotePath, media.MediaMeta{Filename: "video-note.mp4", ContentType: "video/mp4", Source: "telegram"}))
			lastMediaName = "video-note.mp4"
			if content != "" {
				content += "\n"
			}
			content += "[video note]"
		}
	}

	if message.Animation != nil {
		animationType := strings.TrimSpace(message.Animation.MimeType)
		animationExt := ".mp4"
		switch {
		case strings.Contains(strings.ToLower(animationType), "gif"):
			animationExt = ".gif"
		case strings.Contains(strings.ToLower(animationType), "webm"):
			animationExt = ".webm"
		}
		animationPath := c.downloadFile(ctx, message.Animation.FileID, animationExt)
		if animationPath != "" {
			if animationType == "" {
				animationType = "video/mp4"
			}
			animationName := "animation" + animationExt
			mediaPaths = append(mediaPaths, storeMedia(animationPath, media.MediaMeta{Filename: animationName, ContentType: animationType, Source: "telegram"}))
			lastMediaName = animationName
			if content != "" {
				content += "\n"
			}
			content += "[animation]"
		}
	}

	if message.Document != nil {
		docPath := c.downloadFile(ctx, message.Document.FileID, "")
		if docPath != "" {
			docName := strings.TrimSpace(message.Document.FileName)
			if docName == "" {
				docName = filepath.Base(docPath)
			}
			docType := strings.TrimSpace(message.Document.MimeType)
			mediaPaths = append(mediaPaths, storeMedia(docPath, media.MediaMeta{Filename: docName, ContentType: docType, Source: "telegram"}))
			lastMediaName = docName
			if content != "" {
				content += "\n"
			}
			content += "[file: " + docName + "]"
		}
	}

	if len(mediaPaths) == 0 && shouldReuseRecentAttachment(content) {
		if ref, name, ok := c.recallRecentMedia(chatIDStr, 30*time.Minute); ok {
			mediaPaths = append(mediaPaths, ref)
			if strings.TrimSpace(name) != "" {
				content += "\n[file: " + name + "]"
			} else {
				content += "\n[file: attachment]"
			}
			lastMediaName = name
		}
	}

	if len(mediaPaths) > 0 {
		c.rememberRecentMedia(chatIDStr, mediaPaths[len(mediaPaths)-1], lastMediaName)
		if wantsRomanianTranslation(content) {
			content += "\n[task: translate attached file to Romanian directly after reading it]"
			if wantsPDFReturn(content) {
				content += "\n[task: return the translated result as a PDF file]"
			}
		}
	}

	if content == "" {
		content = "[empty message]"
	}

	// In group chats, apply unified group trigger filtering
	if message.Chat.Type != "private" {
		isMentioned := c.isBotMentioned(message)
		if isMentioned {
			content = c.stripBotMention(content)
		}
		respond, cleaned := c.ShouldRespondInGroup(isMentioned, content)
		if !respond {
			return nil
		}
		content = cleaned
	}

	logger.InfoCF("telegram", "Inbound message", map[string]any{
		"sender_id":   sender.CanonicalID,
		"chat_id":     fmt.Sprintf("%d", chatID),
		"message_id":  messageIDStr,
		"media_count": len(mediaPaths),
		"preview":     utils.Truncate(content, 160),
	})
	logger.DebugCF("telegram", "Received message", map[string]any{
		"sender_id": sender.CanonicalID,
		"chat_id":   fmt.Sprintf("%d", chatID),
		"preview":   utils.Truncate(content, 50),
	})

	// Placeholder is now auto-triggered by BaseChannel.HandleMessage via PlaceholderCapable

	peerKind := "direct"
	peerID := fmt.Sprintf("%d", user.ID)
	if message.Chat.Type != "private" {
		peerKind = "group"
		peerID = fmt.Sprintf("%d", chatID)
	}

	peer := bus.Peer{Kind: peerKind, ID: peerID}
	messageID := fmt.Sprintf("%d", message.MessageID)

	metadata := map[string]string{
		"user_id":          fmt.Sprintf("%d", user.ID),
		"username":         user.Username,
		"first_name":       user.FirstName,
		"is_group":         fmt.Sprintf("%t", message.Chat.Type != "private"),
		"user_profile_key": sender.CanonicalID,
	}

	if adminBody, adminRequested := parseAdminEscalation(content, c.botUsername()); adminRequested {
		if !c.isAdminUser(platformID) {
			return c.sendPlainText(ctx, chatID, "Admin escalation denied for this Telegram account.")
		}
		if strings.TrimSpace(adminBody) == "" {
			return c.sendPlainText(ctx, chatID, "Usage: /admin <instruction>")
		}
		content = adminBody
		metadata["admin_escalated"] = "true"
		metadata["admin_user_id"] = platformID
		metadata["admin_user"] = sender.CanonicalID
	}

	c.HandleMessage(c.ctx,
		peer,
		messageID,
		platformID,
		fmt.Sprintf("%d", chatID),
		content,
		mediaPaths,
		metadata,
		sender,
	)
	return nil
}

func (c *TelegramChannel) isAdminUser(userID string) bool {
	for _, allowed := range c.config.Channels.Telegram.AdminIDs {
		if strings.TrimSpace(allowed) == strings.TrimSpace(userID) {
			return true
		}
	}
	return false
}

func (c *TelegramChannel) botUsername() string {
	if c == nil || c.bot == nil {
		return ""
	}
	return c.bot.Username()
}

func parseAdminEscalation(content, botUsername string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}

	lower := strings.ToLower(trimmed)
	switch {
	case lower == "/admin":
		return "", true
	case strings.HasPrefix(lower, "/admin "):
		return strings.TrimSpace(trimmed[len("/admin "):]), true
	case botUsername != "" && (lower == strings.ToLower("/admin@"+botUsername)):
		return "", true
	case botUsername != "" && strings.HasPrefix(lower, strings.ToLower("/admin@"+botUsername+" ")):
		prefixLen := len("/admin@" + botUsername + " ")
		return strings.TrimSpace(trimmed[prefixLen:]), true
	case lower == "!admin":
		return "", true
	case strings.HasPrefix(lower, "!admin "):
		return strings.TrimSpace(trimmed[len("!admin "):]), true
	default:
		return "", false
	}
}

func (c *TelegramChannel) sendPlainText(ctx context.Context, chatID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	logger.InfoCF("telegram", "Outbound text", map[string]any{
		"chat_id":     fmt.Sprintf("%d", chatID),
		"content_len": len(text),
		"preview":     utils.Truncate(text, 160),
	})
	_, err := c.bot.SendMessage(ctx, tu.Message(tu.ID(chatID), text))
	return err
}

func (c *TelegramChannel) downloadPhoto(ctx context.Context, fileID string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		logger.ErrorCF("telegram", "Failed to get photo file", map[string]any{
			"error": err.Error(),
		})
		return ""
	}

	return c.downloadFileWithInfo(file, ".jpg")
}

func (c *TelegramChannel) downloadFileWithInfo(file *telego.File, ext string) string {
	if file.FilePath == "" {
		return ""
	}

	url := c.bot.FileDownloadURL(file.FilePath)
	logger.DebugCF("telegram", "File URL", map[string]any{"url": url})

	// Use FilePath as filename for better identification
	filename := file.FilePath + ext
	return utils.DownloadFile(url, filename, utils.DownloadOptions{
		LoggerPrefix: "telegram",
	})
}

func (c *TelegramChannel) downloadFile(ctx context.Context, fileID, ext string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		logger.ErrorCF("telegram", "Failed to get file", map[string]any{
			"error": err.Error(),
		})
		return ""
	}

	return c.downloadFileWithInfo(file, ext)
}

func parseChatID(chatIDStr string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(chatIDStr, "%d", &id)
	return id, err
}

func markdownToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	codeBlocks := extractCodeBlocks(text)
	text = codeBlocks.text

	inlineCodes := extractInlineCodes(text)
	text = inlineCodes.text

	text = reHeading.ReplaceAllString(text, "$1")

	text = reBlockquote.ReplaceAllString(text, "$1")

	text = escapeHTML(text)

	text = reLink.ReplaceAllString(text, `<a href="$2">$1</a>`)

	text = reBoldItalicStar.ReplaceAllString(text, "<b><i>$1</i></b>")

	text = reBoldStar.ReplaceAllString(text, "<b>$1</b>")

	text = reBoldUnder.ReplaceAllString(text, "<b>$1</b>")

	text = reItalic.ReplaceAllStringFunc(text, func(s string) string {
		match := reItalic.FindStringSubmatch(s)
		if len(match) < 2 {
			return s
		}
		return "<i>" + match[1] + "</i>"
	})

	text = replaceSingleAsteriskItalics(text)

	text = reStrike.ReplaceAllString(text, "<s>$1</s>")

	text = reListItem.ReplaceAllString(text, "• ")

	for i, code := range inlineCodes.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00IC%d\x00", i), fmt.Sprintf("<code>%s</code>", escaped))
	}

	for i, code := range codeBlocks.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(
			text,
			fmt.Sprintf("\x00CB%d\x00", i),
			fmt.Sprintf("<pre><code>%s</code></pre>", escaped),
		)
	}

	return text
}

type codeBlockMatch struct {
	text  string
	codes []string
}

func extractCodeBlocks(text string) codeBlockMatch {
	matches := reCodeBlock.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	i := 0
	text = reCodeBlock.ReplaceAllStringFunc(text, func(m string) string {
		placeholder := fmt.Sprintf("\x00CB%d\x00", i)
		i++
		return placeholder
	})

	return codeBlockMatch{text: text, codes: codes}
}

type inlineCodeMatch struct {
	text  string
	codes []string
}

func extractInlineCodes(text string) inlineCodeMatch {
	matches := reInlineCode.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	i := 0
	text = reInlineCode.ReplaceAllStringFunc(text, func(m string) string {
		placeholder := fmt.Sprintf("\x00IC%d\x00", i)
		i++
		return placeholder
	})

	return inlineCodeMatch{text: text, codes: codes}
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

func replaceSingleAsteriskItalics(text string) string {
	var b strings.Builder
	b.Grow(len(text))

	for i := 0; i < len(text); {
		if text[i] != '*' || (i+1 < len(text) && text[i+1] == '*') {
			b.WriteByte(text[i])
			i++
			continue
		}

		closeIdx := -1
		for j := i + 1; j < len(text); j++ {
			if text[j] == '\n' {
				break
			}
			if text[j] != '*' {
				continue
			}
			if j+1 < len(text) && text[j+1] == '*' {
				continue
			}
			closeIdx = j
			break
		}
		if closeIdx == -1 {
			b.WriteByte(text[i])
			i++
			continue
		}

		content := text[i+1 : closeIdx]
		if strings.TrimSpace(content) == "" {
			b.WriteByte(text[i])
			i++
			continue
		}

		b.WriteString("<i>")
		b.WriteString(content)
		b.WriteString("</i>")
		i = closeIdx + 1
	}

	return b.String()
}

// isBotMentioned checks if the bot is mentioned in the message via entities.
func (c *TelegramChannel) isBotMentioned(message *telego.Message) bool {
	text, entities := telegramEntityTextAndList(message)
	if text == "" || len(entities) == 0 {
		return false
	}

	botUsername := ""
	if c.bot != nil {
		botUsername = c.bot.Username()
	}
	runes := []rune(text)

	for _, entity := range entities {
		entityText, ok := telegramEntityText(runes, entity)
		if !ok {
			continue
		}

		switch entity.Type {
		case telego.EntityTypeMention:
			if botUsername != "" && strings.EqualFold(entityText, "@"+botUsername) {
				return true
			}
		case telego.EntityTypeTextMention:
			if botUsername != "" && entity.User != nil && strings.EqualFold(entity.User.Username, botUsername) {
				return true
			}
		case telego.EntityTypeBotCommand:
			if isBotCommandEntityForThisBot(entityText, botUsername) {
				return true
			}
		}
	}
	return false
}

func telegramEntityTextAndList(message *telego.Message) (string, []telego.MessageEntity) {
	if message.Text != "" {
		return message.Text, message.Entities
	}
	return message.Caption, message.CaptionEntities
}

func telegramEntityText(runes []rune, entity telego.MessageEntity) (string, bool) {
	if entity.Offset < 0 || entity.Length <= 0 {
		return "", false
	}
	end := entity.Offset + entity.Length
	if entity.Offset >= len(runes) || end > len(runes) {
		return "", false
	}
	return string(runes[entity.Offset:end]), true
}

func isBotCommandEntityForThisBot(entityText, botUsername string) bool {
	if !strings.HasPrefix(entityText, "/") {
		return false
	}
	command := strings.TrimPrefix(entityText, "/")
	if command == "" {
		return false
	}

	at := strings.IndexRune(command, '@')
	if at == -1 {
		// A bare /command delivered to this bot is intended for this bot.
		return true
	}

	mentionUsername := command[at+1:]
	if mentionUsername == "" || botUsername == "" {
		return false
	}
	return strings.EqualFold(mentionUsername, botUsername)
}

// stripBotMention removes the @bot mention from the content.
func (c *TelegramChannel) stripBotMention(content string) string {
	botUsername := c.bot.Username()
	if botUsername == "" {
		return content
	}
	// Case-insensitive replacement
	re := regexp.MustCompile(`(?i)@` + regexp.QuoteMeta(botUsername))
	content = re.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

func shouldReuseRecentAttachment(content string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(content))
	if trimmed == "" {
		return false
	}

	if trimmed == "to ro" || trimmed == "to romanian" || trimmed == "into romanian" {
		return true
	}

	if strings.Contains(trimmed, "romanian") || strings.Contains(trimmed, "to ro") {
		return len(strings.Fields(trimmed)) <= 8
	}

	return false
}

func wantsRomanianTranslation(content string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(content))
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "to ro") || strings.Contains(trimmed, "romanian")
}

func wantsPDFReturn(content string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(content))
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "send back as pdf") || strings.Contains(trimmed, "as pdf") || strings.Contains(trimmed, "pdf export")
}

func (c *TelegramChannel) rememberRecentMedia(chatID, mediaRef, mediaName string) {
	if strings.TrimSpace(chatID) == "" || strings.TrimSpace(mediaRef) == "" {
		return
	}

	c.mediaMu.Lock()
	defer c.mediaMu.Unlock()

	c.lastMediaRefByChat[chatID] = mediaRef
	c.lastMediaNameByChat[chatID] = mediaName
	c.lastMediaSeenByChat[chatID] = time.Now()
}

func (c *TelegramChannel) recallRecentMedia(chatID string, ttl time.Duration) (string, string, bool) {
	if strings.TrimSpace(chatID) == "" {
		return "", "", false
	}

	c.mediaMu.Lock()
	defer c.mediaMu.Unlock()

	seenAt, ok := c.lastMediaSeenByChat[chatID]
	if !ok {
		return "", "", false
	}
	if ttl > 0 && time.Since(seenAt) > ttl {
		delete(c.lastMediaRefByChat, chatID)
		delete(c.lastMediaNameByChat, chatID)
		delete(c.lastMediaSeenByChat, chatID)
		return "", "", false
	}

	ref := c.lastMediaRefByChat[chatID]
	if strings.TrimSpace(ref) == "" {
		return "", "", false
	}
	return ref, c.lastMediaNameByChat[chatID], true
}
