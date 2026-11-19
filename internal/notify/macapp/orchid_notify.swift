import Cocoa
import UserNotifications

// orchid-notify: posts a macOS notification attributed to the Orchid bundle and,
// when the user clicks it, opens the deep-link URL it carries (the dashboard
// focused on the finished session).
//
// Usage: orchid-notify [title] [body] [url]
//   - with title/body  -> post a notification (userInfo.url = url)
//   - no args          -> setup-only OR a click relaunch: register + wait briefly
//                         for a click response, open its url, then exit.
final class AppDelegate: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    func applicationDidFinishLaunching(_ note: Notification) {
        let center = UNUserNotificationCenter.current()
        center.delegate = self

        let args = CommandLine.arguments
        let title = args.count > 1 ? args[1] : ""
        let body = args.count > 2 ? args[2] : ""
        let url = args.count > 3 ? args[3] : ""

        center.requestAuthorization(options: [.alert, .sound]) { _, _ in
            if title.isEmpty && body.isEmpty {
                // Setup-only launch, or a relaunch from a notification click: the
                // click response (if any) arrives on the delegate shortly. Give it
                // a moment, then exit.
                DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { exit(0) }
                return
            }
            let content = UNMutableNotificationContent()
            content.title = title
            content.body = body
            content.sound = .default
            if !url.isEmpty { content.userInfo = ["url": url] }
            let req = UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)
            center.add(req) { _ in
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { exit(0) }
            }
        }
    }

    // Click on the notification -> open the deep link. If the dashboard is
    // installed as a PWA for this origin, open it there (with the manifest's
    // launch_handler "navigate-existing", Chrome focuses the already-open window
    // and navigates it instead of spawning a new one); otherwise the browser.
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                didReceive response: UNNotificationResponse,
                                withCompletionHandler completionHandler: @escaping () -> Void) {
        if let s = response.notification.request.content.userInfo["url"] as? String,
           let u = URL(string: s) {
            openDeepLink(u)
        }
        completionHandler()
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { exit(0) }
    }

    private func openDeepLink(_ url: URL) {
        let ws = NSWorkspace.shared
        // If installed as a PWA, deliver the target via its registered
        // web+orchid: protocol — a plain `open <https-url>` only reopens the PWA
        // at start_url (navigate-existing drops the query/fragment). The protocol
        // launch carries the value, so the PWA navigates the existing window to
        // it. Falls back to the browser when no PWA / no `s` value.
        if installedOrchidPWA() != nil,
           let comps = URLComponents(url: url, resolvingAgainstBaseURL: false),
           let s = comps.queryItems?.first(where: { $0.name == "s" })?.value,
           let enc = s.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed),
           let proto = URL(string: "web+orchid://" + enc) {
            ws.open(proto)
            return
        }
        ws.open(url) // not installed as a PWA -> default browser
    }

    // installedOrchidPWA locates the orchid PWA app installed in the default
    // browser. Chromium PWAs are real .app bundles under ~/Applications (often in
    // a "<Browser> Apps.localized" folder) with a bundle id "<browserId>.app.<hash>"
    // — they do NOT register as URL handlers, so we find them on disk.
    private func installedOrchidPWA() -> URL? {
        let ws = NSWorkspace.shared
        guard let browser = ws.urlForApplication(toOpen: URL(string: "https://example.com")!),
              let browserId = Bundle(url: browser)?.bundleIdentifier else { return nil }
        let prefix = browserId + ".app." // PWA installed in the default browser
        let fm = FileManager.default
        let appsRoot = fm.homeDirectoryForCurrentUser.appendingPathComponent("Applications")
        var dirs = [appsRoot]
        if let entries = try? fm.contentsOfDirectory(at: appsRoot, includingPropertiesForKeys: nil) {
            for e in entries where e.lastPathComponent.hasSuffix("Apps.localized") { dirs.append(e) }
        }
        for dir in dirs {
            guard let apps = try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil) else { continue }
            for app in apps where app.pathExtension == "app" {
                guard let id = Bundle(url: app)?.bundleIdentifier else { continue }
                let nameMatch = app.deletingPathExtension().lastPathComponent.lowercased().contains("orchid")
                if id.hasPrefix(prefix) && nameMatch { return app }
            }
        }
        return nil
    }

    // Allow the banner to show even if Orchid happens to be the active app.
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                willPresent notification: UNNotification,
                                withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .sound])
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.accessory) // no dock icon / menu bar
app.run()
