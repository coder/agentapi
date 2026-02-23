<script setup lang="ts">
defineProps<{
  type?: 'tip' | 'warning' | 'danger' | 'note'
  title?: string
}>()
</script>

<template>
  <div class="callout" :class="type || 'note'">
    <div class="callout-title">
      <strong>{{ title || type }}</strong>
    </div>
    <div class="callout-content">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.callout {
  padding: 16px;
  border-radius: 8px;
  margin: 16px 0;
  border-left: 4px solid;
}

.callout.tip {
  background: rgba(34, 197, 94, 0.1);
  border-color: #22c55e;
}

.callout.warning {
  background: rgba(234, 179, 8, 0.1);
  border-color: #eab308;
}

.callout.danger {
  background: rgba(239, 68, 68, 0.1);
  border-color: #ef4444;
}

.callout.note {
  background: rgba(59, 130, 246, 0.1);
  border-color: #3b82f6;
}

.callout-title {
  font-weight: 600;
  margin-bottom: 8px;
}

.callout-title strong {
  text-transform: capitalize;
}

.callout-content {
  line-height: 1.6;
}
</style>
