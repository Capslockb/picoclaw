package commands

// BuiltinDefinitions returns all built-in command definitions.
// Each command is a single-word command, compatible with Telegram's /command format.
func BuiltinDefinitions() []Definition {
	return []Definition{
		// Core
		startCommand(),
		helpCommand(),
		versionCommand(),
		pingCommand(),
		clearCommand(),

		// Info
		modelCommand(),   // /model [name] — show or switch model
		modelsCommand(),  // /models — list configured model
		channelCommand(), // /channel [name] — show or check channel
		channelsCommand(), // /channels — list enabled channels
		agentsCommand(), // /agents — list registered agents
		toolsCommand(),  // /tools — list available tools

		// Google / GWS
		gloginCommand(),   // /glogin [antigravity|gemini]
		gstatusCommand(),  // /gstatus
		glogoutCommand(),  // /glogout [antigravity|gemini|all]
		gprojectCommand(), // /gproject <project-id>
		gmailCommand(),    // /gmail [list|search|unread|read]
		driveCommand(),    // /drive [list|search|docs|sheets]
		docsCommand(),     // /docs [create|open|list]
		calCommand(),      // /cal [today|week|month]
		sheetsCommand(),   // /sheets [list|search]

		// Platform
		qrCommand(),        // /qr — WhatsApp QR code
		vpsloginCommand(),  // /vpslogin <password>
		acpCommand(),       // /acp <action>

		// Legacy stubs (kept for backward compat, redirect users)
		showCommand(),
		listCommand(),
		switchCommand(),
		checkCommand(),
		whatsappCommand(),
		vpsCommand(),
	}
}
