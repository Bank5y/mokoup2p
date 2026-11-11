<script setup>
import { onMounted, ref } from 'vue'
import {
  createUser, deleteUser, listUsers, resetUserPassword, setUserAdmin, setUserDisabled,
} from '../backend.js'
import { t, errorText } from '../i18n/index.js'

const props = defineProps({ status: { type: Object, required: true } })
// status-lost: the daemon is no longer logged in as an administrator.
const emit = defineEmits(['status-lost'])

const users = ref(null)
const error = ref('')
const message = ref('')
const busy = ref(false)

const newUsername = ref('')
const newPassword = ref('')

// resetting is the user whose password is being reset, or null.
const resetting = ref(null)
const resetPassword = ref('')

function fail(err) {
  const code = String(err?.message ?? err)
  if (['forbidden', 'not_logged_in', 'token_invalid', 'account_disabled'].includes(code)) {
    emit('status-lost')
  }
  error.value = errorText(err)
}

async function load() {
  try {
    users.value = await listUsers()
  } catch (err) {
    fail(err)
  }
}

// run performs one administrator action, then reloads the list.
async function run(action, success) {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await action()
    if (success) message.value = success
    await load()
    return true
  } catch (err) {
    fail(err)
    return false
  } finally {
    busy.value = false
  }
}

async function submitCreate() {
  const username = newUsername.value
  const ok = await run(() => createUser(username, newPassword.value), t('admin.created', { username }))
  if (ok) newUsername.value = newPassword.value = ''
}

function startReset(user) {
  resetting.value = user
  resetPassword.value = ''
}

async function submitReset() {
  const user = resetting.value
  const ok = await run(() => resetUserPassword(user.id, resetPassword.value),
    t('admin.resetDone', { username: user.username }))
  if (ok) resetting.value = null
}

function toggleDisabled(user) {
  run(() => setUserDisabled(user.id, !user.disabled))
}

function toggleAdmin(user) {
  run(() => setUserAdmin(user.id, !user.isAdmin))
}

function remove(user) {
  if (!window.confirm(t('admin.confirmDelete', { username: user.username }))) return
  run(() => deleteUser(user.id))
}

onMounted(load)
</script>

<template>
  <section class="page">
    <h2 class="section-title">{{ t('admin.createTitle') }}</h2>
    <form class="form form-inline" @submit.prevent="submitCreate">
      <input v-model.trim="newUsername" class="input" type="text" required autocomplete="off"
             :placeholder="t('admin.username')" />
      <input v-model="newPassword" class="input" type="password" required autocomplete="new-password"
             :placeholder="t('admin.initialPassword')" />
      <button class="button button-primary" type="submit" :disabled="busy">{{ t('admin.create') }}</button>
    </form>

    <p v-if="error" class="notice notice-error">{{ error }}</p>
    <p v-if="message" class="notice notice-ok">{{ message }}</p>

    <form v-if="resetting" class="form form-inline" @submit.prevent="submitReset">
      <span class="field-label">{{ t('admin.resetPrompt', { username: resetting.username }) }}</span>
      <input v-model="resetPassword" class="input" type="password" required autocomplete="new-password" />
      <button class="button button-primary" type="submit" :disabled="busy">{{ t('admin.confirmReset') }}</button>
      <button class="button" type="button" @click="resetting = null">{{ t('admin.cancel') }}</button>
    </form>

    <h2 class="section-title">{{ t('admin.title') }}</h2>
    <p v-if="!users" class="field-label">{{ t('admin.loading') }}</p>
    <table v-else class="table">
      <thead>
        <tr>
          <th>{{ t('admin.colUsername') }}</th>
          <th>{{ t('admin.colRole') }}</th>
          <th>{{ t('admin.colState') }}</th>
          <th>{{ t('admin.colActions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="user in users" :key="user.id">
          <td>
            {{ user.username }}
            <span v-if="user.username === props.status.username" class="field-label">{{ t('admin.you') }}</span>
          </td>
          <td>{{ user.isAdmin ? t('admin.roleAdmin') : t('admin.roleUser') }}</td>
          <td :class="{ 'text-error': user.disabled }">
            {{ user.disabled ? t('admin.stateDisabled') : t('admin.stateEnabled') }}
          </td>
          <td class="actions">
            <button class="button button-small" type="button" :disabled="busy" @click="startReset(user)">
              {{ t('admin.resetPassword') }}
            </button>
            <button class="button button-small" type="button" :disabled="busy" @click="toggleDisabled(user)">
              {{ user.disabled ? t('admin.enable') : t('admin.disable') }}
            </button>
            <button class="button button-small" type="button" :disabled="busy" @click="toggleAdmin(user)">
              {{ user.isAdmin ? t('admin.revokeAdmin') : t('admin.makeAdmin') }}
            </button>
            <button class="button button-small button-danger" type="button" :disabled="busy" @click="remove(user)">
              {{ t('admin.delete') }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
