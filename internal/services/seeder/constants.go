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

package seeder

const defaultCSS = `body {
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Arial, sans-serif;
  font-size: 16px;
  line-height: 1.6;
  color: #111827;
  background-color: #f9fafb;
}

.email-wrapper {
  width: 100%;
  padding: 32px 0;
  background-color: #f9fafb;
}

.email-container {
  max-width: 600px;
  margin: 0 auto;
  background-color: #ffffff;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid #e5e7eb;
}

.email-header {
  background: linear-gradient(135deg, #7e22ce, #a855f7);
  color: #ffffff;
  padding: 36px;
  text-align: center;
}

.email-header h1 {
  margin: 0 0 6px;
  font-size: 26px;
  font-weight: 700;
}

.email-header p {
  margin: 0;
  font-size: 14px;
  opacity: 0.9;
}

.email-body {
  padding: 32px 36px;
}

.email-body h2 {
  margin-top: 0;
  margin-bottom: 16px;
  color: #111827;
  font-size: 20px;
}

.email-body p {
  margin: 0 0 16px;
  color: #4b5563;
}

.feature-list {
  list-style: none;
  padding: 0;
  margin: 20px 0 24px;
}

.feature-list li {
  padding: 10px 0;
  border-bottom: 1px solid #e5e7eb;
  color: #4b5563;
  font-size: 15px;
}

.feature-list li:last-child {
  border-bottom: none;
}

.btn {
  display: inline-block;
  padding: 14px 26px;
  background: linear-gradient(135deg, #7e22ce, #a855f7);
  color: #ffffff;
  text-decoration: none;
  border-radius: 6px;
  font-weight: 600;
  font-size: 14px;
}

.email-footer {
  padding: 24px 36px;
  text-align: center;
  font-size: 13px;
  color: #9ca3af;
  border-top: 1px solid #e5e7eb;
  background-color: #f9fafb;
}

.email-footer a {
  color: #9333ea;
  text-decoration: none;
}
`

const defaultHTMLTemplate = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Welcome to Posta</h1>
      <p>Your self-hosted email delivery platform</p>
    </div>
    <div class="email-body">
      <h2>Hello {{name}},</h2>
      <p>Welcome to <strong>{{product}}</strong>. Your account is ready and you can start sending emails immediately.</p>
      <p>{{product}} provides the following capabilities:</p>
      <ul class="feature-list">
        {{range features}}
        <li>{{.}}</li>
        {{end}}
      </ul>
      <p>Helpful resources:</p>
      <ul>
        {{range links}}
        <li><a href="{{.url}}">{{.title}}</a></li>
        {{end}}
      </ul>
      <p style="text-align:center;margin:32px 0;">
        <a href="{{docs}}" class="btn">View API Documentation</a>
      </p>
      <p>Best regards,<br/>The {{company}} Team</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}} — Licensed under Apache 2.0</p>
    </div>
  </div>
</div>`

const defaultTextTemplate = `Hello {{name}},

Welcome to {{product}}.

Your account is ready and you can begin sending emails immediately.

Key capabilities:
{{range features}}
- {{.}}
{{end}}

Helpful resources:
{{range links}}
- {{.title}}: {{.url}}
{{end}}

Best regards,
The {{company}} Team

© {{year}} {{company}} — Licensed under Apache 2.0
`

const defaultHTMLTemplateFr = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Bienvenue sur Posta</h1>
      <p>Votre plateforme d'envoi d'e-mails auto-hébergée</p>
    </div>
    <div class="email-body">
      <h2>Bonjour {{name}},</h2>
      <p>Bienvenue sur <strong>{{product}}</strong>. Votre compte est prêt et vous pouvez commencer à envoyer des emails immédiatement.</p>
      <p>{{product}} offre les fonctionnalités suivantes :</p>
      <ul class="feature-list">
        {{range features}}
        <li>{{.}}</li>
        {{end}}
      </ul>
      <p>Ressources utiles :</p>
      <ul>
        {{range links}}
        <li><a href="{{.url}}">{{.title}}</a></li>
        {{end}}
      </ul>
      <p style="text-align:center;margin:32px 0;">
        <a href="{{docs}}" class="btn">Voir la documentation</a>
      </p>
      <p>Cordialement,<br/>L'équipe {{company}}</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}} — Licence Apache 2.0</p>
    </div>
  </div>
</div>`

const defaultTextTemplateFr = `Bonjour {{name}},

Bienvenue sur {{product}}.

Votre compte est prêt et vous pouvez commencer à envoyer des emails immédiatement.

Fonctionnalités principales :
{{range features}}
- {{.}}
{{end}}

Ressources utiles :
{{range links}}
- {{.title}} : {{.url}}
{{end}}

Cordialement,
L'équipe {{company}}

© {{year}} {{company}} — Licence Apache 2.0
`
