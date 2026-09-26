//
//  PlatriumApp.swift
//  Platrium
//
//  Created by Atheesh Thirumalairajan on 9/25/26.
//

import SwiftUI
import PlatriumSDK
import OSLog

@main
struct PlatriumApp: App {
    private func testSDK() async {
        do {
            let client = try PlatriumClient(baseUrl: "http://localhost:3000/api")
            let download_session = try await client.files().createDownloadSession(fileId: "-WzQIsIbA5KUfDZVYMd83")
            Logger().critical("FileName: \(download_session.fileName())")
        } catch {
            print(error)
        }
    }
    
    var body: some Scene {
        WindowGroup {
            ContentView()
                .task {
                    await testSDK()
                }
        }
    }
}
