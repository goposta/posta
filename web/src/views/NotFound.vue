<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="not-found">
    <div class="not-found-content">
      <h1>404</h1>
      <p>The page you're looking for doesn't exist.</p>
      <button class="btn btn-primary" @click="router.push('/')">Go to Dashboard</button>
    </div>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-primary);
}

.not-found-content {
  text-align: center;
  padding: 48px 24px;
}

.not-found-content h1 {
  font-size: 96px;
  font-weight: 700;
  color: var(--text-muted);
  line-height: 1;
  margin-bottom: 12px;
}

.not-found-content p {
  font-size: 16px;
  color: var(--text-secondary);
  margin-bottom: 24px;
}
</style>
