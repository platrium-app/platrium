import SwiftUI

enum SidebarSelection: Hashable {
    case home
    case myDrive(id: String)
    case sharedDrivesOverview
    case sharedDrive(id: String)
    case sharedWithMe
    case trash
    case settings
    case folder(id: String)
}
