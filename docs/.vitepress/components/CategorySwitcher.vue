<template>
  <div class="category-switcher">
    <a v-for="item in items" :key="item.link" :href="item.link" class="category-link">{{ item.text }}</a>
  </div>
</template>

<script setup>
const items = [
  { text: 'Wiki', link: '/wiki/' },
  { text: 'Development Guide', link: '/development-guide/' },
  { text: 'Document Index', link: '/document-index/' },
  { text: 'API', link: '/api/' },
  { text: 'Roadmap', link: '/roadmap/' }
]
</script>
