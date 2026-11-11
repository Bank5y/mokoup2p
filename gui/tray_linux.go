package main

import "github.com/godbus/dbus/v5"

// trayAvailable reports whether the desktop shows tray icons. On Linux the
// tray is a StatusNotifierItem, which needs a watcher on the session bus;
// stock GNOME has none, and the icon would silently not appear.
func trayAvailable() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	var hasOwner bool
	err = conn.BusObject().
		Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").
		Store(&hasOwner)
	return err == nil && hasOwner
}
