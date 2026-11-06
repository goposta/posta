// The widget's stylesheet, adopted into its shadow root. Sites theme it with
// --posta-chat-* custom properties set on any ancestor; the workspace's accent
// colour arrives as --pc-accent from a second, generated sheet.

export const css = `
:host{all:initial}
.pc{--a:var(--posta-chat-accent,var(--pc-accent,#7c3aed));--at:var(--posta-chat-accent-text,#fff);
--bg:var(--posta-chat-bg,#fff);--fg:var(--posta-chat-text,#111827);--mut:var(--posta-chat-muted,#6b7280);
--bd:var(--posta-chat-border,#e5e7eb);--in:var(--posta-chat-input-bg,#fff);--sub:var(--posta-chat-bubble,#f3f4f6);
--r:var(--posta-chat-radius,14px);--err:#b91c1c;
position:fixed;bottom:20px;right:20px;z-index:var(--posta-chat-z-index,2147483000);
font-family:var(--posta-chat-font,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif);font-size:14px;line-height:1.45;color:var(--fg);
display:flex;flex-direction:column;align-items:flex-end;gap:12px}
.pc.left{right:auto;left:20px;align-items:flex-start}
@media (prefers-color-scheme:dark){.pc{--bg:var(--posta-chat-bg,#111827);--fg:var(--posta-chat-text,#f9fafb);
--mut:var(--posta-chat-muted,#9ca3af);--bd:var(--posta-chat-border,#374151);--in:var(--posta-chat-input-bg,#1f2937);--sub:var(--posta-chat-bubble,#1f2937);--err:#fca5a5}}
*{box-sizing:border-box}
button,input,textarea{font:inherit;color:inherit}
.launcher{width:56px;height:56px;border-radius:50%;border:0;background:var(--a);color:var(--at);cursor:pointer;
display:grid;place-items:center;box-shadow:0 6px 20px rgba(0,0,0,.2);position:relative;transition:transform .15s}
.launcher:hover{transform:scale(1.05)}
.launcher:focus-visible,.send:focus-visible,.x:focus-visible{outline:3px solid var(--a);outline-offset:3px}
.launcher svg{width:26px;height:26px}
.badge{position:absolute;top:-2px;right:-2px;min-width:20px;height:20px;padding:0 6px;border-radius:10px;background:#dc2626;color:#fff;font-size:12px;font-weight:600;display:grid;place-items:center}
.panel{width:370px;max-width:calc(100vw - 40px);height:560px;max-height:calc(100vh - 110px);background:var(--bg);
border:1px solid var(--bd);border-radius:var(--r);box-shadow:0 12px 40px rgba(0,0,0,.18);display:flex;flex-direction:column;overflow:hidden;
animation:pc-in .18s ease-out}
@keyframes pc-in{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.head{background:var(--a);color:var(--at);padding:14px 16px;display:flex;align-items:center;gap:10px}
.head img{width:32px;height:32px;border-radius:50%;object-fit:cover;background:rgba(255,255,255,.2)}
.head h2{margin:0;font-size:16px;font-weight:600;flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.x{background:transparent;border:0;color:inherit;cursor:pointer;width:32px;height:32px;border-radius:8px;display:grid;place-items:center}
.x:hover{background:rgba(255,255,255,.15)}
.log{flex:1;overflow-y:auto;padding:16px;display:flex;flex-direction:column;gap:8px}
.msg{max-width:82%;padding:9px 12px;border-radius:14px;white-space:pre-wrap;overflow-wrap:anywhere;background:var(--sub)}
.msg.visitor{align-self:flex-end;background:var(--a);color:var(--at);border-bottom-right-radius:4px}
.msg.agent,.msg.greet{align-self:flex-start;border-bottom-left-radius:4px}
.msg.system{align-self:center;background:transparent;color:var(--mut);font-size:12px}
.msg a{color:inherit;text-decoration:underline}
.who{display:block;font-size:11px;color:var(--mut);margin-bottom:2px}
.state{font-size:11px;color:var(--mut);align-self:flex-end;margin-top:-4px}
.state.failed{color:var(--err);cursor:pointer;background:none;border:0;padding:0}
.state.seen{margin-top:-4px}
.typing{align-self:flex-start;font-style:italic}
.att-img{display:block;max-width:100%;max-height:220px;border-radius:8px;margin:2px 0}
.att-file{display:inline-flex;gap:6px;align-items:center;background:none;border:1px solid currentColor;border-radius:8px;padding:6px 10px;cursor:pointer;font-size:13px;margin:2px 0}
.clip{background:none;border:1px solid var(--bd);border-radius:10px;width:40px;height:40px;cursor:pointer;display:grid;place-items:center;color:var(--mut);flex:none}
.clip:disabled{opacity:.4;cursor:default}
.note{align-self:center;text-align:center;color:var(--mut);font-size:12px;padding:4px 8px}
.form{border-top:1px solid var(--bd);padding:10px;display:flex;flex-direction:column;gap:8px}
.fields{display:flex;gap:8px}
.fields input{flex:1;min-width:0}
input,textarea{background:var(--in);border:1px solid var(--bd);border-radius:10px;padding:9px 11px;outline:none}
input:focus,textarea:focus{border-color:var(--a)}
.row{display:flex;gap:8px;align-items:flex-end}
textarea{flex:1;resize:none;min-height:40px;max-height:120px}
.send{border:0;border-radius:10px;background:var(--a);color:var(--at);height:40px;padding:0 14px;cursor:pointer;font-weight:600}
.send:disabled{opacity:.5;cursor:default}
.err{color:var(--err);font-size:12px}
.foot{display:flex;justify-content:center;padding:2px 0 9px}
.foot a{display:inline-flex;align-items:center;gap:5px;font-size:12px;color:var(--mut);text-decoration:none}
.foot a:hover{color:var(--fg)}
.foot strong{font-weight:700}
.foot .mk{display:inline-flex}
.sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
@media (max-width:480px){.pc.opened{inset:0;align-items:stretch}.pc.opened .panel{width:100%;max-width:none;height:100%;max-height:none;border-radius:0}.pc.opened .launcher{display:none}}
@media (prefers-reduced-motion:reduce){.panel{animation:none}.launcher{transition:none}}
`

// adopt styles a shadow root without a <style> element where the browser
// allows it, so the widget works under a strict style-src CSP.
export function adopt(root: ShadowRoot, sheets: string[]): void {
  if ('adoptedStyleSheets' in root && typeof CSSStyleSheet === 'function') {
    try {
      root.adoptedStyleSheets = sheets.map((text) => {
        const s = new CSSStyleSheet()
        s.replaceSync(text)
        return s
      })
      return
    } catch {
      // Older engines: fall through.
    }
  }
  for (const text of sheets) {
    const el = document.createElement('style')
    el.textContent = text
    root.appendChild(el)
  }
}

const ACCENT = /^#[0-9a-fA-F]{6}$/

// accentSheet sets the workspace's accent colour, which the server already
// validated; it is checked again because it lands in a stylesheet.
export function accentSheet(color?: string): string {
  return color && ACCENT.test(color) ? `.pc{--pc-accent:${color}}` : ''
}
