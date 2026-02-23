import DefaultTheme from 'vitepress/theme'
import './custom.css'
import Callout from './components/Callout.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('Callout', Callout)
  }
}
