// isAdminLost tells whether an error means the daemon is no longer logged in
// as an administrator, so the page should refresh the status.
export function isAdminLost(err) {
  const code = String(err?.message ?? err)
  return ['forbidden', 'not_logged_in', 'token_invalid', 'account_disabled'].includes(code)
}
