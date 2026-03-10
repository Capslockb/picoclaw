package commands

import (
	"context"
	"fmt"
)

func qrCommand() Definition {
	return Definition{
		Name:        "qr",
		Description: "Get the WhatsApp pairing QR code",
		Usage:       "/qr",
		Handler: func(ctx context.Context, req Request, rt *Runtime) error {
			ch, ok := rt.GetChannel("whatsapp_native")
			if !ok {
				return req.Reply("whatsapp_native channel is not enabled.")
			}

			type qrProvider interface {
				GetLastQR() string
			}

			qp, ok := ch.(qrProvider)
			if !ok {
				return req.Reply("whatsapp_native channel does not support QR retrieval.")
			}

			qr := qp.GetLastQR()
			if qr == "" {
				return req.Reply("No QR code available yet. Wait for the channel to initialize.")
			}

			return req.Reply(fmt.Sprintf("Scan this QR code: %s", qr))
		},
	}
}

// whatsappCommand kept as a stub for backward compat.
func whatsappCommand() Definition {
	return Definition{
		Name:        "whatsapp",
		Description: "Alias: use /qr",
		Usage:       "/whatsapp",
		Handler: func(_ context.Context, req Request, _ *Runtime) error {
			return req.Reply("Use /qr to get the WhatsApp pairing QR code.")
		},
	}
}
