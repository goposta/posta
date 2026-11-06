<script setup lang="ts">
import { useConfirm } from '../composables/useConfirm'

const { visible, options, handleConfirm, handleCancel } = useConfirm()

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') handleCancel()
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="modal-overlay" @click.self="handleCancel" @keydown="onKeydown">
      <div class="modal confirm-dialog" role="alertdialog" aria-modal="true">
        <div class="modal-header">
          <div class="confirm-header-content">
            <div class="confirm-icon" :class="`confirm-icon-${options.variant}`">
              <svg v-if="options.variant === 'danger'" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
              <svg v-else-if="options.variant === 'warning'" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                <line x1="12" y1="9" x2="12" y2="13" />
                <line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <svg v-else width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="16" x2="12" y2="12" />
                <line x1="12" y1="8" x2="12.01" y2="8" />
              </svg>
            </div>
            <h3>{{ options.title }}</h3>
          </div>
        </div>
        <div class="modal-body">
          <p class="confirm-message">{{ options.message }}</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="handleCancel" ref="cancelBtn">
            {{ options.cancelText }}
          </button>
          <button
            class="btn"
            :class="options.variant === 'danger' ? 'btn-danger' : options.variant === 'warning' ? 'btn-primary' : 'btn-primary'"
            @click="handleConfirm"
          >
            {{ options.confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-dialog {
  max-width: 440px;
}

.confirm-header-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.confirm-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border-radius: 10px;
  flex-shrink: 0;
}

.confirm-icon-danger {
  background: var(--danger-50);
  color: var(--danger-600);
}

.confirm-icon-warning {
  background: var(--warning-50);
  color: var(--warning-600);
}

.confirm-icon-info {
  background: var(--primary-50);
  color: var(--primary-600);
}

.confirm-message {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
}
</style>
