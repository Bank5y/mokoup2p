<script setup>
import { t } from '../i18n/index.js'

defineProps({ status: { type: Object, required: true } })
</script>

<template>
  <section class="home">
    <header class="topbar">
      <span class="topbar-title">{{ t('app.name') }}</span>
      <span class="topbar-meta">{{ t('home.loggedInAs', { username: status.username }) }}</span>
      <span class="topbar-meta">{{ t('home.device', { deviceName: status.deviceName }) }}</span>
    </header>
    <div class="empty">{{ t('home.empty') }}</div>
  </section>
</template>
