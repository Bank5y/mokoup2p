<script setup>
import { onMounted, ref } from 'vue'
import { adminDeleteRoom, allRooms } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import { isAdminLost } from './adminErrors.js'

const emit = defineEmits(['status-lost'])

const rooms = ref(null)
const error = ref('')
const busy = ref(false)

function fail(err) {
  if (isAdminLost(err)) emit('status-lost')
  error.value = errorText(err)
}

async function load() {
  try {
    rooms.value = await allRooms()
  } catch (err) {
    fail(err)
  }
}

async function remove(room) {
  if (!window.confirm(t('adminRooms.confirmDelete', { name: room.name, code: room.code }))) return
  busy.value = true
  error.value = ''
  try {
    await adminDeleteRoom(room.code)
    await load()
  } catch (err) {
    fail(err)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="panel-body">
    <div class="panel-header">
      <h2 class="section-title">{{ t('adminRooms.title') }}</h2>
      <button class="button button-small" type="button" @click="load">{{ t('adminRooms.reload') }}</button>
    </div>
    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <p v-if="!rooms" class="field-label">{{ t('admin.loading') }}</p>
    <p v-else-if="rooms.length === 0" class="field-label">{{ t('adminRooms.empty') }}</p>
    <table v-else class="table">
      <thead>
        <tr>
          <th>{{ t('adminRooms.colName') }}</th>
          <th>{{ t('adminRooms.colCode') }}</th>
          <th>{{ t('adminRooms.colOwner') }}</th>
          <th>{{ t('adminRooms.colOnline') }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="room in rooms" :key="room.code">
          <td>{{ room.name }}</td>
          <td class="room-facts-ip">{{ room.code }}</td>
          <td>{{ room.owner }}</td>
          <td>{{ room.onlineMembers }}</td>
          <td class="table-actions">
            <button class="button button-small button-danger" type="button" :disabled="busy" @click="remove(room)">
              {{ t('admin.delete') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
