import Foundation
#if canImport(UIKit)
import UIKit
#endif

/// What the server shows for this installation under Devices & Apps.
public struct DeviceDescriptor: Sendable, Equatable {
    /// `IOS`, `MACOS`, ... (the engine stores it as given).
    public var platform: String
    /// e.g. "Platrium on Atheesh's MacBook Pro".
    public var name: String
    public var appVersion: String

    public init(platform: String, name: String, appVersion: String) {
        self.platform = platform
        self.name = name
        self.appVersion = appVersion
    }

    public static var current: DeviceDescriptor {
        let version = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? ""
        #if os(iOS)
        let device = UIDevice.current.name
        let platform = "IOS"
        #else
        let device = Host.current().localizedName ?? "Mac"
        let platform = "MACOS"
        #endif
        return DeviceDescriptor(platform: platform, name: "Platrium on \(device)", appVersion: version)
    }
}
