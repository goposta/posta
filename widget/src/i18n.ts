// The widget's own words. Greetings and titles come from the workspace.

const en = {
  open: 'Open chat',
  close: 'Close chat',
  title: 'Chat with us',
  name: 'Your name',
  email: 'Your email',
  placeholder: 'Write a message…',
  send: 'Send',
  sending: 'Sending…',
  failed: 'Not sent. Tap to retry.',
  received: 'Thanks, your message was sent.',
  closed: 'This conversation is closed. Write again to reopen it.',
  you: 'You',
  agent: 'Support',
  tooLong: 'This message is too long.',
  rateLimited: 'You are sending messages too quickly. Please wait a moment.',
  error: 'Something went wrong. Please try again.',
  emailInvalid: 'Please enter a valid email address.',
  required: 'Please fill in this field.',
  powered: 'Powered by',
  offline: "We're away right now. Leave a message and we'll reply by email.",
  typing: 'is typing…',
  someone: 'Support',
  seen: 'Seen',
  attach: 'Attach a file',
  attachFirst: 'Send a message first',
  fileTooLarge: 'Files can be at most 5 MB.',
  home: 'Home',
  messages: 'Messages',
  help: 'Help',
  back: 'Back',
  sendMessage: 'Send us a message',
  continueChat: 'Continue the conversation',
  replyByEmail: "We'll reply by email",
  noConversations: 'No conversations yet.',
  newTab: '(opens in a new tab)',
  search: 'Search help',
  noResults: 'Nothing matches.',
  closedConv: 'Closed',
  newConversation: 'New conversation',
  replyMinutes: 'Typically replies in a few minutes',
  replyHours: 'Typically replies in a few hours',
  replyDay: 'Typically replies within a day',
  away: "We're away right now.",
  backAt: "We're away. Back at {t}",
  backOn: "We're away. Back {d} at {t}",
  andTeam: '{names} and the team',
  dismiss: 'Dismiss',
  previewQuestion: 'Do you ship to Canada?',
  previewAnswer: 'Yes, in 3 to 5 working days.',
}

export type Strings = typeof en

const fr: Strings = {
  open: 'Ouvrir le chat',
  close: 'Fermer le chat',
  title: 'Discutons',
  name: 'Votre nom',
  email: 'Votre e-mail',
  placeholder: 'Écrivez un message…',
  send: 'Envoyer',
  sending: 'Envoi…',
  failed: 'Non envoyé. Touchez pour réessayer.',
  received: 'Merci, votre message a été envoyé.',
  closed: 'Cette conversation est fermée. Écrivez pour la rouvrir.',
  you: 'Vous',
  agent: 'Support',
  tooLong: 'Ce message est trop long.',
  rateLimited: 'Vous envoyez des messages trop vite. Patientez un instant.',
  error: 'Une erreur est survenue. Réessayez.',
  emailInvalid: 'Saisissez une adresse e-mail valide.',
  required: 'Veuillez remplir ce champ.',
  powered: 'Propulsé par',
  offline: 'Nous sommes absents. Laissez un message, nous répondrons par e-mail.',
  typing: 'écrit…',
  someone: 'Le support',
  seen: 'Vu',
  attach: 'Joindre un fichier',
  attachFirst: "Envoyez d'abord un message",
  fileTooLarge: 'Les fichiers font 5 Mo au plus.',
  home: 'Accueil',
  messages: 'Messages',
  help: 'Aide',
  back: 'Retour',
  sendMessage: 'Envoyez-nous un message',
  continueChat: 'Reprendre la conversation',
  replyByEmail: 'Nous répondrons par e-mail',
  noConversations: 'Aucune conversation pour le moment.',
  newTab: '(s’ouvre dans un nouvel onglet)',
  search: "Chercher dans l'aide",
  noResults: 'Aucun résultat.',
  closedConv: 'Fermée',
  newConversation: 'Nouvelle conversation',
  replyMinutes: 'Répond généralement en quelques minutes',
  replyHours: 'Répond généralement en quelques heures',
  replyDay: 'Répond généralement sous un jour',
  away: 'Nous sommes absents pour le moment.',
  backAt: 'Nous sommes absents. De retour à {t}',
  backOn: 'Nous sommes absents. De retour {d} à {t}',
  andTeam: "{names} et l'équipe",
  dismiss: 'Fermer',
  previewQuestion: 'Livrez-vous au Canada ?',
  previewAnswer: 'Oui, en 3 à 5 jours ouvrés.',
}

export function strings(lang = navigator.language): Strings {
  return lang.toLowerCase().startsWith('fr') ? fr : en
}

export function fill(template: string, values: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (m, k) => values[k] ?? m)
}

// A greeting may hold {name}; without a known name the placeholder and the space before it go.
export function personalise(greeting: string | undefined, name?: string): string {
  return (greeting || '').replace(/\s*\{name\}/g, name ? ` ${name}` : '')
}

// The visitor's own clock: a time today, a weekday within the week, a date beyond.
export function backText(t: Strings, iso: string, now = new Date(), lang = navigator.language): string {
  const at = new Date(iso)
  if (isNaN(at.getTime())) return t.away
  const time = new Intl.DateTimeFormat(lang, { hour: '2-digit', minute: '2-digit' }).format(at)
  if (at.toDateString() === now.toDateString()) return fill(t.backAt, { t: time })
  const days = (at.getTime() - now.getTime()) / 86400000
  const d = new Intl.DateTimeFormat(lang, days < 6 ? { weekday: 'long' } : { weekday: 'long', day: 'numeric', month: 'long' }).format(at)
  return fill(t.backOn, { d, t: time })
}

export function namesText(t: Strings, names: string[]): string {
  return fill(t.andTeam, { names: names.join(', ') })
}
