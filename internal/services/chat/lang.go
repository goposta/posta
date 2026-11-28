// Copyright (c) 2026 Jonas Kaninda.
//
// This file is part of Posta.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package chat

import "strings"

// LanguageAuto lets each visitor's browser choose among Languages.
const LanguageAuto = "auto"

// Languages the widget and the follow-up email speak; keep in step with widget/src/i18n.ts.
var Languages = []string{"en", "fr", "es", "pt", "de", "sw", "ln"}

// ValidWidgetLanguage accepts auto or one of Languages.
func ValidWidgetLanguage(lang string) bool {
	return lang == LanguageAuto || supported(lang) != ""
}

// supported maps a tag such as pt-BR to its base language when Posta speaks it, else "".
func supported(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
	for _, l := range Languages {
		if l == base {
			return l
		}
	}
	return ""
}

// ConversationLanguage is the widget's fixed language, else the visitor's, else English.
func ConversationLanguage(widgetLang, visitorLang string) string {
	if l := supported(widgetLang); l != "" {
		return l
	}
	if l := supported(visitorLang); l != "" {
		return l
	}
	return "en"
}

const wordSupport = "Support"

type emailWords struct {
	Support, ReplyHere, ContinueLead, Or, ContinueInChat, Subject string
}

var emailText = map[string]emailWords{
	"en": {wordSupport, "Reply to this email to answer.", "Continue the conversation in the chat", "Or", "continue in the chat", "New reply from %s"},
	"fr": {wordSupport, "Répondez à cet e-mail pour continuer.", "Poursuivez la conversation dans le chat", "Ou", "poursuivez dans le chat", "Nouvelle réponse de %s"},
	"es": {"Soporte", "Responde a este correo para contestar.", "Continúa la conversación en el chat", "O", "continúa en el chat", "Nueva respuesta de %s"},
	"pt": {"Suporte", "Responda a este e-mail para continuar.", "Continue a conversa no chat", "Ou", "continue no chat", "Nova resposta de %s"},
	"de": {wordSupport, "Antworten Sie einfach auf diese E-Mail.", "Setzen Sie das Gespräch im Chat fort", "Oder", "im Chat weiterschreiben", "Neue Antwort von %s"},
	"sw": {"Msaada", "Jibu barua pepe hii ili kuendelea.", "Endelea na mazungumzo kwenye gumzo", "Au", "endelea kwenye gumzo", "Jibu jipya kutoka %s"},
	"ln": {"Lisalisi", "Zongisa eyano na e-mail oyo mpo na kokoba.", "Kokoba lisolo na chat", "To", "kokoba na chat", "Eyano ya sika euti na %s"},
}

func wordsFor(lang string) emailWords {
	if w, ok := emailText[supported(lang)]; ok {
		return w
	}
	return emailText["en"]
}
