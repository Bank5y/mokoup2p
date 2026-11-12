<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { kick, leave, members as listMembers } from '../backend.js'
import { t, errorText } from '../i18n/index.js'
import RoomCode from '../components/RoomCode.vue'

// room is status.room: the room this device is in. connected and revision
// come from the status too; a new revision may mean the members changed.
const props = defineProps({
  room: { type: Object, required: true },
  connected: { type: Boolean, default: true },
  revision: { type: Number, default: 0 },
  // isOwner: this device's user owns the room and may kick members.
  isOwner: { type: Boolean, default: false },
})
const emit = defineEmits(['left'])

const busy = ref(false)
const error = ref('')
const members = ref([])
const membersError = ref('')

async function loadMembers() {
  try {
    members.value = await listMembers()
    membersError.value = ''
  } catch (err) {
    membersError.value = errorText(err)
  }
}

// Path switches arrive as a new revision; latencies are refreshed on a timer.
let timer = null
onMounted(() => {
  loadMembers()
  timer = setInterval(loadMembers, 5000)
})
onUnmounted(() => clearInterval(timer))
watch(() => props.revision, loadMembers)

function latencyText(ms) {
  if (!ms) return ''
  return t('currentRoom.latency', { ms: ms < 10 ? ms.toFixed(1) : Math.round(ms) })
}

async function kickMember(m) {
  if (!window.confirm(t('currentRoom.kickConfirm', { member: `${m.username} / ${m.deviceName}` }))) return
  error.value = ''
  try {
    await kick(m.id)
    await loadMembers()
  } catch (err) {
    error.value = errorText(err)
  }
}

async function leaveRoom() {
  busy.value = true
  error.value = ''
  try {
    emit('left', await leave())
  } catch (err) {
    error.value = errorText(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2 class="panel-title">{{ room.name }}</h2>
    <p v-if="!connected" class="notice notice-error">{{ t('currentRoom.disconnected') }}</p>
    <dl class="room-facts">
      <dt>{{ t('room.code') }}</dt>
      <dd><RoomCode :code="room.code" /></dd>
      <dt>{{ t('currentRoom.virtualIp') }}</dt>
      <dd class="room-facts-ip">{{ room.virtualIp }}</dd>
      <dt>{{ t('currentRoom.subnet') }}</dt>
      <dd>{{ room.subnet }}</dd>
      <dt>{{ t('currentRoom.owner') }}</dt>
      <dd>{{ room.owner }}</dd>
    </dl>
    <h3 class="section-title">{{ t('currentRoom.membersTitle', { count: members.length }) }}</h3>
    <p v-if="membersError" class="notice notice-error">{{ membersError }}</p>
    <table class="table members">
      <thead>
        <tr>
          <th>{{ t('currentRoom.colMember') }}</th>
          <th>{{ t('currentRoom.colVirtualIp') }}</th>
          <th>{{ t('currentRoom.colPath') }}</th>
          <th>{{ t('currentRoom.colLatency') }}</th>
          <th v-if="isOwner"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in members" :key="m.id">
          <td>
            {{ m.username }} / {{ m.deviceName }}
            <span v-if="m.self" class="member-self">{{ t('currentRoom.self') }}</span>
          </td>
          <td class="room-facts-ip">{{ m.virtualIp }}</td>
          <td>
            <span v-if="!m.self" class="tag" :class="m.path ? `tag-${m.path}` : ''">
              {{ t(`currentRoom.path.${m.path || 'unknown'}`) }}
            </span>
          </td>
          <td>{{ m.self ? '' : latencyText(m.latencyMs) }}</td>
          <td v-if="isOwner" class="table-actions">
            <button v-if="!m.self" class="button button-small" type="button" @click="kickMember(m)">
              {{ t('currentRoom.kick') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <div class="actions">
      <button class="button" type="button" :disabled="busy" @click="leaveRoom">
        {{ busy ? t('currentRoom.leaving') : t('currentRoom.leave') }}
      </button>
    </div>
  </section>
</template>
