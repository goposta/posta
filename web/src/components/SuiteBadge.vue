<script setup lang="ts">
import { useId } from 'vue'

// Mask ids are document-global, so each instance needs its own.
const uid = useId()
const square = 'M104,0 H408 A104,104 0 0 1 512,104 V408 A104,104 0 0 1 408,512 H104 A104,104 0 0 1 0,408 V104 A104,104 0 0 1 104,0 Z'
</script>

<template>
  <div class="suite" role="img" aria-label="Part of Posta Suite: Posta, Posta Mail and Posta Webmail">
    <span class="suite-part">Part of</span>
    <span class="suite-icons" aria-hidden="true">
      <svg viewBox="0 0 512 512" width="22" height="22">
        <title>Posta</title>
        <path class="sq" :d="square" />
        <path class="gl" fill-rule="evenodd" d="M168,110 H344 A20,20 0 0 1 364,130 V262 A20,20 0 0 1 344,282 H168 A20,20 0 0 1 148,262 V130 A20,20 0 0 1 168,110 Z M191,172 H321 A13,13 0 0 1 334,185 V185 A13,13 0 0 1 321,198 H191 A13,13 0 0 1 178,185 V185 A13,13 0 0 1 191,172 Z M78,250 L256,377 L434,250 L434,380 A36,36 0 0 1 398,416 L114,416 A36,36 0 0 1 78,380 Z" />
      </svg>
      <svg viewBox="0 0 512 512" width="22" height="22">
        <title>Posta Mail</title>
        <path class="sq" :d="square" />
        <mask :id="`${uid}-mail`" maskUnits="userSpaceOnUse" x="0" y="0" width="512" height="512">
          <rect width="512" height="512" fill="#fff" />
          <polyline points="99,206 231,308 363,206" fill="none" stroke="#000" stroke-width="30" stroke-linecap="round" stroke-linejoin="round" />
          <circle cx="383" cy="184" r="74" fill="#000" />
        </mask>
        <g class="gl">
          <g :mask="`url(#${uid}-mail)`"><rect x="81" y="186" width="300" height="216" rx="40" /></g>
          <circle cx="383" cy="184" r="48" />
        </g>
      </svg>
      <svg viewBox="0 0 512 512" width="22" height="22">
        <title>Posta Webmail</title>
        <path class="sq" :d="square" />
        <mask :id="`${uid}-webmail`" maskUnits="userSpaceOnUse" x="0" y="0" width="512" height="512">
          <rect width="512" height="512" fill="#fff" />
          <rect x="80" y="166" width="352" height="20" fill="#000" />
          <circle cx="136" cy="135" r="14" fill="#000" />
          <circle cx="178" cy="135" r="14" fill="#000" />
          <rect x="158" y="226" width="196" height="136" rx="22" fill="none" stroke="#000" stroke-width="24" />
          <polyline points="168,238 256,302 344,238" fill="none" stroke="#000" stroke-width="24" stroke-linecap="round" stroke-linejoin="round" />
        </mask>
        <g class="gl">
          <g :mask="`url(#${uid}-webmail)`"><rect x="80" y="104" width="352" height="304" rx="44" /></g>
        </g>
      </svg>
    </span>
    <span class="suite-name">Posta <b>Suite</b></span>
  </div>
</template>

<style scoped>
.suite {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  color: var(--text-primary);
}
.suite-part {
  color: var(--text-muted);
  font-size: 13px;
}
.suite-icons {
  display: flex;
  gap: 5px;
}
.suite-icons .sq { fill: currentColor; }
/* The glyphs read as cut-outs, so they take the page background in either theme. */
.suite-icons .gl { fill: var(--bg-primary); }
.suite-name {
  font-weight: 700;
  letter-spacing: -0.02em;
}
.suite-name b { color: var(--primary-600); }
:global([data-theme="dark"]) .suite-name b { color: var(--primary-400); }
</style>
