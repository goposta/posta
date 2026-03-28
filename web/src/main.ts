import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './assets/styles.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)

// Register workspace interceptor (injects X-Posta-Workspace-Id header)
import './stores/workspace'

app.mount('#app')
