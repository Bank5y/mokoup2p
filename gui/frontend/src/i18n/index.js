// All UI text comes from the locale resource files. Only zh-CN exists for now.
import zhCN from './zh-CN.json'

const messages = zhCN

// t('home.loggedInAs', { username: 'alice' }) looks up a dotted key and fills
// {placeholders}. A missing key renders as the key itself so it is noticed.
export function t(key, params = {}) {
  const text = key.split('.').reduce((node, part) => node?.[part], messages)
  if (typeof text !== 'string') return key
  return text.replace(/\{(\w+)\}/g, (_, name) => params[name] ?? `{${name}}`)
}

// errorText maps an error code from the backend to its message.
export function errorText(err) {
  const code = typeof err === 'string' ? err : err?.message
  const key = `errors.${code}`
  return t(key) === key ? t('errors.unknown') : t(key)
}
