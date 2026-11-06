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
}

export function strings(lang = navigator.language): Strings {
  return lang.toLowerCase().startsWith('fr') ? fr : en
}
