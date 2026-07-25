import { createApp } from 'vue'

import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'

import 'element-plus/dist/index.css'

import App from './App.vue'
import { initFocusGuard } from '@/utils/focusGuard'

import '@/styles/global.css'


const app = createApp(App)

app.use(createPinia())
app.use(ElementPlus)

initFocusGuard()
app.mount('#app')
