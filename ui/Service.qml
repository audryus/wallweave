// Service.qml
// Plugin "service" entry point: omarchy-shell mounts it ONCE per shell
// while the plugin is enabled, so every bar widget (one per monitor) shares
// this single backend process instead of starting its own. Widgets reach it
// through bar.shell.serviceFor("audryus.wallweave") — see Main.qml.
import QtQuick

Backend { }
