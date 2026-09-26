import SwiftUI

struct SettingsView: View {
    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: "gear")
                .font(.system(size: 48))
                .foregroundStyle(.tint)
            Text("Settings")
                .font(.title)
                .bold()
            Text("Server connection and app settings will be managed here.")
                .foregroundStyle(.secondary)
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .navigationTitle("Settings")
    }
}

#Preview {
    SettingsView()
}
