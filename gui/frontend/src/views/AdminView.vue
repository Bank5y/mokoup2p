<script setup>
import { ref } from 'vue'
import { t } from '../i18n/index.js'
import AdminUsers from './AdminUsers.vue'
import AdminRooms from './AdminRooms.vue'
import AdminSettings from './AdminSettings.vue'

defineProps({ status: { type: Object, required: true } })
// status-lost: the daemon is no longer logged in as an administrator.
const emit = defineEmits(['status-lost'])

// tab is 'users', 'rooms' or 'settings'.
const tab = ref('users')
</script>

<template>
  <section class="page">
    <nav class="tabs">
      <button v-for="name in ['users', 'rooms', 'settings']" :key="name" type="button"
              class="nav-item" :class="{ active: tab === name }" @click="tab = name">
        {{ t(`admin.tab.${name}`) }}
      </button>
    </nav>
    <AdminUsers v-if="tab === 'users'" :status="status" @status-lost="emit('status-lost')" />
    <AdminRooms v-else-if="tab === 'rooms'" @status-lost="emit('status-lost')" />
    <AdminSettings v-else @status-lost="emit('status-lost')" />
  </section>
</template>
