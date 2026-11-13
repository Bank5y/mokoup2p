// Shared update state: the banner shows `available` whenever it is set; the
// startup check and the settings page's "check now" both fill it.
import { ref } from 'vue'
import { checkUpdate, updateSettings, setUpdateSettings } from './backend.js'

// available is the UpdateInfo of a newer, not ignored release, or null.
export const available = ref(null)

function offer(info) {
  available.value = info.available && !info.ignored ? info : null
}

// checkAtStartup checks once if the user has not turned automatic checks
// off. Failures are silent: an offline machine should not nag at startup.
export async function checkAtStartup() {
  try {
    const s = await updateSettings()
    if (s.autoCheck) offer(await checkUpdate())
  } catch {
    // ignored
  }
}

// checkNow checks regardless of the setting and returns the UpdateInfo
// (rejecting with the error code on failure).
export async function checkNow() {
  const info = await checkUpdate()
  offer(info)
  return info
}

// ignore stops offering version, keeping the automatic-check setting.
export async function ignore(version) {
  const s = await updateSettings()
  await setUpdateSettings(s.autoCheck, version)
  available.value = null
}
