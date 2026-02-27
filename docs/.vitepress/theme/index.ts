import DefaultTheme from 'vitepress/theme'
import CategorySwitcher from '../components/CategorySwitcher.vue'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('CategorySwitcher', CategorySwitcher)
  }
}
