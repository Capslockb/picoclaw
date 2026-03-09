package telegram

import "testing"

func TestParseAdminEscalation(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		botUsername string
		wantBody    string
		wantOK      bool
	}{
		{name: "plain slash admin", content: "/admin restart nginx", wantBody: "restart nginx", wantOK: true},
		{name: "mention slash admin", content: "/admin@picoclawbot status", botUsername: "PicoclawBot", wantBody: "status", wantOK: true},
		{name: "bang admin", content: "!admin inspect route", wantBody: "inspect route", wantOK: true},
		{name: "empty admin", content: "/admin", wantBody: "", wantOK: true},
		{name: "normal message", content: "hello there", wantBody: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBody, gotOK := parseAdminEscalation(tt.content, tt.botUsername)
			if gotBody != tt.wantBody || gotOK != tt.wantOK {
				t.Fatalf("parseAdminEscalation(%q, %q) = (%q, %v), want (%q, %v)", tt.content, tt.botUsername, gotBody, gotOK, tt.wantBody, tt.wantOK)
			}
		})
	}
}
