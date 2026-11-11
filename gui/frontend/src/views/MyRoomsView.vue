<script setup>
import { onMounted, ref } from 'vue'
import { myRooms, renameRoom, setRoomPassword, deleteRoom } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import RoomCode from '../components/RoomCode.vue'

const emit = defineEmits(['create'])

const rooms = ref(null)
const error = ref('')
// editing is { code, kind: 'rename' | 'password' | 'delete' } while a row is open.
const editing = ref(null)
const input = ref('')
const busy = ref(false)

async function load() {
  try {
    rooms.value = await myRooms()
    error.value = ''
  } catch (err) {
    error.value = errorText(err)
  }
}

function open(room, kind) {
  editing.value = { code: room.code, kind }
  input.value = kind === 'rename' ? room.name : ''
  error.value = ''
}

function close() {
  editing.value = null
  input.value = ''
}

function replace(updated) {
  rooms.value = rooms.value.map((r) => (r.code === updated.code ? updated : r))
}

async function run(action) {
  busy.value = true
  error.value = ''
  try {
    await action()
    close()
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}

const rename = (room) => run(async () => replace(await renameRoom(room.code, input.value)))
const savePassword = (room, password) => run(async () => replace(await setRoomPassword(room.code, password)))
const remove = (room) => run(async () => {
  await deleteRoom(room.code)
  rooms.value = rooms.value.filter((r) => r.code !== room.code)
})

const isOpen = (room, kind) => editing.value?.code === room.code && editing.value?.kind === kind

onMounted(load)
</script>

<template>
  <section class="panel">
    <div class="panel-header">
      <h2 class="panel-title">{{ t('myRooms.title') }}</h2>
      <button class="button button-primary" type="button" @click="emit('create')">{{ t('home.createRoom') }}</button>
    </div>
    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <p v-if="!rooms" class="hint">{{ t('common.loading') }}</p>
    <p v-else-if="rooms.length === 0" class="empty">{{ t('myRooms.empty') }}</p>
    <ul v-else class="room-list">
      <li v-for="room in rooms" :key="room.code" class="room-item">
        <div class="room-item-main">
          <span class="room-name">{{ room.name }}</span>
          <span class="tag">{{ room.hasPassword ? t('room.hasPassword') : t('room.noPassword') }}</span>
          <span class="room-meta">{{ t('room.subnet', { subnet: room.subnet }) }}</span>
        </div>
        <div class="room-item-row">
          <RoomCode :code="room.code" />
          <span class="room-item-actions">
            <button class="button button-small" type="button" @click="open(room, 'rename')">{{ t('myRooms.rename') }}</button>
            <button class="button button-small" type="button" @click="open(room, 'password')">{{ t('myRooms.password') }}</button>
            <button class="button button-small button-danger" type="button" @click="open(room, 'delete')">{{ t('myRooms.delete') }}</button>
          </span>
        </div>

        <form v-if="isOpen(room, 'rename')" class="inline-form" @submit.prevent="rename(room)">
          <input v-model.trim="input" class="input" type="text" required maxlength="64" :placeholder="t('myRooms.newName')" />
          <button class="button button-primary" type="submit" :disabled="busy">{{ t('myRooms.save') }}</button>
          <button class="button" type="button" @click="close">{{ t('myRooms.cancel') }}</button>
        </form>

        <form v-if="isOpen(room, 'password')" class="inline-form" @submit.prevent="savePassword(room, input)">
          <input v-model="input" class="input" type="password" required autocomplete="new-password" :placeholder="t('myRooms.newPassword')" />
          <button class="button button-primary" type="submit" :disabled="busy">{{ t('myRooms.setPassword') }}</button>
          <button v-if="room.hasPassword" class="button" type="button" :disabled="busy" @click="savePassword(room, '')">
            {{ t('myRooms.removePassword') }}
          </button>
          <button class="button" type="button" @click="close">{{ t('myRooms.cancel') }}</button>
        </form>

        <div v-if="isOpen(room, 'delete')" class="inline-form">
          <span class="inline-form-text">{{ t('myRooms.confirmDelete', { name: room.name }) }}</span>
          <button class="button button-danger" type="button" :disabled="busy" @click="remove(room)">{{ t('myRooms.confirm') }}</button>
          <button class="button" type="button" @click="close">{{ t('myRooms.cancel') }}</button>
        </div>
      </li>
    </ul>
  </section>
</template>
