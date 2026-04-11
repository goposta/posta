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

// ======== Password Reset Templates ========

const passwordResetHTMLTemplate = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Password Reset</h1>
      <p>Reset your account password</p>
    </div>
    <div class="email-body">
      <h2>Hello {{name}},</h2>
      <p>We received a request to reset your password. Click the button below to choose a new one. This link will expire in <strong>{{expiry}}</strong>.</p>
      <p style="text-align:center;margin:32px 0;">
        <a href="{{resetLink}}" class="btn">Reset Password</a>
      </p>
      <p>If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.</p>
      <p>Best regards,<br/>The {{company}} Team</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
    </div>
  </div>
</div>`

const passwordResetTextTemplate = `Hello {{name}},

We received a request to reset your password.

Click the link below to choose a new password. This link will expire in {{expiry}}.

Reset your password: {{resetLink}}

If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.

Best regards,
The {{company}} Team

© {{year}} {{company}}
`

const passwordResetHTMLTemplateFr = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Réinitialisation du mot de passe</h1>
      <p>Réinitialisez le mot de passe de votre compte</p>
    </div>
    <div class="email-body">
      <h2>Bonjour {{name}},</h2>
      <p>Nous avons reçu une demande de réinitialisation de votre mot de passe. Cliquez sur le bouton ci-dessous pour en choisir un nouveau. Ce lien expirera dans <strong>{{expiry}}</strong>.</p>
      <p style="text-align:center;margin:32px 0;">
        <a href="{{resetLink}}" class="btn">Réinitialiser le mot de passe</a>
      </p>
      <p>Si vous n'avez pas demandé de réinitialisation de mot de passe, vous pouvez ignorer cet email en toute sécurité. Votre mot de passe restera inchangé.</p>
      <p>Cordialement,<br/>L'équipe {{company}}</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
    </div>
  </div>
</div>`

const passwordResetTextTemplateFr = `Bonjour {{name}},

Nous avons reçu une demande de réinitialisation de votre mot de passe.

Cliquez sur le lien ci-dessous pour choisir un nouveau mot de passe. Ce lien expirera dans {{expiry}}.

Réinitialiser votre mot de passe : {{resetLink}}

Si vous n'avez pas demandé de réinitialisation de mot de passe, vous pouvez ignorer cet email en toute sécurité. Votre mot de passe restera inchangé.

Cordialement,
L'équipe {{company}}

© {{year}} {{company}}
`

// ======== Order Confirmation Templates ========

const orderConfirmationHTMLTemplate = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Order Confirmed</h1>
      <p>Thank you for your purchase</p>
    </div>
    <div class="email-body">
      <h2>Hello {{name}},</h2>
      <p>Your order <strong>#{{orderNumber}}</strong> has been confirmed. Here is a summary of your purchase:</p>
      <table width="100%" cellpadding="0" cellspacing="0" style="margin:20px 0;border-collapse:collapse;">
        <tr style="border-bottom:2px solid #e5e7eb;">
          <th style="text-align:left;padding:10px 0;color:#111827;">Item</th>
          <th style="text-align:center;padding:10px 0;color:#111827;">Qty</th>
          <th style="text-align:right;padding:10px 0;color:#111827;">Price</th>
        </tr>
        {{range items}}
        <tr style="border-bottom:1px solid #e5e7eb;">
          <td style="padding:10px 0;color:#4b5563;">{{.name}}</td>
          <td style="text-align:center;padding:10px 0;color:#4b5563;">{{.qty}}</td>
          <td style="text-align:right;padding:10px 0;color:#4b5563;">{{.price}}</td>
        </tr>
        {{end}}
        <tr>
          <td colspan="2" style="padding:12px 0;font-weight:700;color:#111827;">Total</td>
          <td style="text-align:right;padding:12px 0;font-weight:700;color:#111827;">{{total}}</td>
        </tr>
      </table>
      <p>We will notify you once your order has shipped.</p>
      <p>Best regards,<br/>The {{company}} Team</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
    </div>
  </div>
</div>`

const orderConfirmationTextTemplate = `Hello {{name}},

Your order #{{orderNumber}} has been confirmed. Here is a summary of your purchase:

{{range items}}
- {{.name}} (x{{.qty}}): {{.price}}
{{end}}

Total: {{total}}

We will notify you once your order has shipped.

Best regards,
The {{company}} Team

© {{year}} {{company}}
`

const orderConfirmationHTMLTemplateFr = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Commande confirmée</h1>
      <p>Merci pour votre achat</p>
    </div>
    <div class="email-body">
      <h2>Bonjour {{name}},</h2>
      <p>Votre commande <strong>#{{orderNumber}}</strong> a été confirmée. Voici un récapitulatif de votre achat :</p>
      <table width="100%" cellpadding="0" cellspacing="0" style="margin:20px 0;border-collapse:collapse;">
        <tr style="border-bottom:2px solid #e5e7eb;">
          <th style="text-align:left;padding:10px 0;color:#111827;">Article</th>
          <th style="text-align:center;padding:10px 0;color:#111827;">Qté</th>
          <th style="text-align:right;padding:10px 0;color:#111827;">Prix</th>
        </tr>
        {{range items}}
        <tr style="border-bottom:1px solid #e5e7eb;">
          <td style="padding:10px 0;color:#4b5563;">{{.name}}</td>
          <td style="text-align:center;padding:10px 0;color:#4b5563;">{{.qty}}</td>
          <td style="text-align:right;padding:10px 0;color:#4b5563;">{{.price}}</td>
        </tr>
        {{end}}
        <tr>
          <td colspan="2" style="padding:12px 0;font-weight:700;color:#111827;">Total</td>
          <td style="text-align:right;padding:12px 0;font-weight:700;color:#111827;">{{total}}</td>
        </tr>
      </table>
      <p>Nous vous informerons dès que votre commande aura été expédiée.</p>
      <p>Cordialement,<br/>L'équipe {{company}}</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
    </div>
  </div>
</div>`

const orderConfirmationTextTemplateFr = `Bonjour {{name}},

Votre commande #{{orderNumber}} a été confirmée. Voici un récapitulatif de votre achat :

{{range items}}
- {{.name}} (x{{.qty}}) : {{.price}}
{{end}}

Total : {{total}}

Nous vous informerons dès que votre commande aura été expédiée.

Cordialement,
L'équipe {{company}}

© {{year}} {{company}}
`

// ======== Newsletter Templates ========

const newsletterHTMLTemplate = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>{{month}} Newsletter</h1>
      <p>The latest updates from {{company}}</p>
    </div>
    <div class="email-body">
      <h2>Hello {{name}},</h2>
      <p>Here are the highlights from this month:</p>
      {{range articles}}
      <div style="margin:24px 0;padding-bottom:20px;border-bottom:1px solid #e5e7eb;">
        <h3 style="margin:0 0 8px;color:#111827;font-size:18px;">{{.title}}</h3>
        <p style="margin:0 0 12px;color:#4b5563;">{{.summary}}</p>
        <a href="{{.url}}" style="color:#9333ea;font-weight:600;text-decoration:none;">Read more →</a>
      </div>
      {{end}}
      <p>Best regards,<br/>The {{company}} Team</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
      <p><a href="{{unsubscribeUrl}}">Unsubscribe</a></p>
    </div>
  </div>
</div>`

const newsletterTextTemplate = `Hello {{name}},

{{month}} Newsletter — The latest updates from {{company}}

{{range articles}}
## {{.title}}
{{.summary}}
Read more: {{.url}}

{{end}}

Best regards,
The {{company}} Team

© {{year}} {{company}}

Unsubscribe: {{unsubscribeUrl}}
`

const newsletterHTMLTemplateFr = `<div class="email-wrapper">
  <div class="email-container">
    <div class="email-header">
      <h1>Newsletter de {{month}}</h1>
      <p>Les dernières nouvelles de {{company}}</p>
    </div>
    <div class="email-body">
      <h2>Bonjour {{name}},</h2>
      <p>Voici les points forts de ce mois-ci :</p>
      {{range articles}}
      <div style="margin:24px 0;padding-bottom:20px;border-bottom:1px solid #e5e7eb;">
        <h3 style="margin:0 0 8px;color:#111827;font-size:18px;">{{.title}}</h3>
        <p style="margin:0 0 12px;color:#4b5563;">{{.summary}}</p>
        <a href="{{.url}}" style="color:#9333ea;font-weight:600;text-decoration:none;">Lire la suite →</a>
      </div>
      {{end}}
      <p>Cordialement,<br/>L'équipe {{company}}</p>
    </div>
    <div class="email-footer">
      <p>© {{year}} {{company}}</p>
      <p><a href="{{unsubscribeUrl}}">Se désabonner</a></p>
    </div>
  </div>
</div>`

const newsletterTextTemplateFr = `Bonjour {{name}},

Newsletter de {{month}} — Les dernières nouvelles de {{company}}

{{range articles}}
## {{.title}}
{{.summary}}
Lire la suite : {{.url}}

{{end}}

Cordialement,
L'équipe {{company}}

© {{year}} {{company}}

Se désabonner : {{unsubscribeUrl}}
`
