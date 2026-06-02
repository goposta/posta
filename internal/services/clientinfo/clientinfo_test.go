/*
 * Copyright 2026 Jonas Kaninda
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package clientinfo

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		ua          string
		wantBrowser string
		wantOS      string
		wantDevice  string
	}{
		{
			name:        "chrome on macos",
			ua:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
			wantBrowser: "Chrome", wantOS: "macOS", wantDevice: "Desktop",
		},
		{
			// Edge carries both "Chrome" and "Safari" tokens; must resolve to Edge.
			name:        "edge on windows",
			ua:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0",
			wantBrowser: "Edge", wantOS: "Windows", wantDevice: "Desktop",
		},
		{
			// Opera also carries a "Chrome" token.
			name:        "opera on windows",
			ua:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 OPR/105.0.0.0",
			wantBrowser: "Opera", wantOS: "Windows", wantDevice: "Desktop",
		},
		{
			// Safari must not be misread as Chrome.
			name:        "safari on iphone",
			ua:          "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
			wantBrowser: "Safari", wantOS: "iOS", wantDevice: "Mobile",
		},
		{
			name:        "chrome on android",
			ua:          "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
			wantBrowser: "Chrome", wantOS: "Android", wantDevice: "Mobile",
		},
		{
			name:        "firefox on linux",
			ua:          "Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0",
			wantBrowser: "Firefox", wantOS: "Linux", wantDevice: "Desktop",
		},
		{
			name:        "ipad is tablet",
			ua:          "Mozilla/5.0 (iPad; CPU OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/604.1",
			wantBrowser: "Safari", wantOS: "iOS", wantDevice: "Tablet",
		},
		{
			name:        "api client",
			ua:          "curl/8.4.0",
			wantBrowser: "API client", wantOS: "Unknown", wantDevice: "Bot",
		},
		{
			name:        "empty",
			ua:          "",
			wantBrowser: "Unknown", wantOS: "Unknown", wantDevice: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.ua)
			if got.Browser != tt.wantBrowser {
				t.Errorf("Browser = %q, want %q", got.Browser, tt.wantBrowser)
			}
			if got.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", got.OS, tt.wantOS)
			}
			if got.Device != tt.wantDevice {
				t.Errorf("Device = %q, want %q", got.Device, tt.wantDevice)
			}
		})
	}
}

func TestLabelAndSignature(t *testing.T) {
	c := Client{Browser: "Chrome", OS: "macOS", Device: "Desktop"}
	if got := c.Label(); got != "Chrome on macOS" {
		t.Errorf("Label() = %q, want %q", got, "Chrome on macOS")
	}
	if got := c.Signature(); got != "Chrome|macOS" {
		t.Errorf("Signature() = %q, want %q", got, "Chrome|macOS")
	}
	if got := (Client{Browser: "Unknown", OS: "Unknown"}).Label(); got != "Unknown device" {
		t.Errorf("Label() = %q, want %q", got, "Unknown device")
	}
}
