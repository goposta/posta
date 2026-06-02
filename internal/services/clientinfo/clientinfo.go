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

import "strings"

const unknown = "Unknown"

// Client is a parsed, display-oriented view of a User-Agent string.
type Client struct {
	Browser string
	OS      string
	Device  string // form factor: "Desktop", "Mobile", "Tablet", or "Bot"
}

func Parse(ua string) Client {
	if strings.TrimSpace(ua) == "" {
		return Client{Browser: unknown, OS: unknown, Device: unknown}
	}
	return Client{
		Browser: detectBrowser(ua),
		OS:      detectOS(ua),
		Device:  detectDevice(ua),
	}
}

func (c Client) Label() string {
	switch {
	case c.Browser == unknown && c.OS == unknown:
		return "Unknown device"
	case c.OS == unknown:
		return c.Browser
	case c.Browser == unknown:
		return c.OS
	default:
		return c.Browser + " on " + c.OS
	}
}

func (c Client) Signature() string {
	return c.Browser + "|" + c.OS
}

func detectBrowser(ua string) string {
	switch {
	case containsAny(ua, "Edg/", "Edge/", "EdgA/", "EdgiOS/"):
		return "Edge"
	case containsAny(ua, "OPR/", "Opera"):
		return "Opera"
	case strings.Contains(ua, "SamsungBrowser"):
		return "Samsung Internet"
	case strings.Contains(ua, "Firefox") || strings.Contains(ua, "FxiOS"):
		return "Firefox"
	case strings.Contains(ua, "CriOS") || strings.Contains(ua, "Chrome") || strings.Contains(ua, "Chromium"):
		return "Chrome"
	case strings.Contains(ua, "Safari"):
		return "Safari"
	case containsAny(ua, "curl/", "Wget", "PostmanRuntime", "Go-http-client", "python-requests"):
		return "API client"
	default:
		return unknown
	}
}

// detectOS checks OS tokens in priority order. iOS/iPadOS must be matched before
// macOS, and Android before Linux, because of overlapping substrings.
func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT"), strings.Contains(ua, "Windows"):
		return "Windows"
	case containsAny(ua, "iPhone", "iPad", "iPod"):
		return "iOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return unknown
	}
}

// detectDevice resolves the form factor.
func detectDevice(ua string) string {
	switch {
	case containsAny(ua, "bot", "Bot", "crawler", "spider", "Go-http-client", "curl/", "python-requests"):
		return "Bot"
	case strings.Contains(ua, "iPad"), strings.Contains(ua, "Tablet"):
		return "Tablet"
	case strings.Contains(ua, "Mobile"), strings.Contains(ua, "iPhone"), strings.Contains(ua, "Android"):
		return "Mobile"
	default:
		return "Desktop"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
