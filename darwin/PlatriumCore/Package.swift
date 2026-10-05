// swift-tools-version:6.1

import PackageDescription

// Code shared by the Platrium app and the FSExtension File Provider extension:
// the server/account store, the Keychain token vault, the sign-in flow and the
// authenticated Apollo/SDK clients.
let package = Package(
  name: "PlatriumCore",
  platforms: [
    .iOS("17.6"),
    .macOS("14.6"),
  ],
  products: [
    .library(name: "PlatriumCore", targets: ["PlatriumCore"]),
  ],
  dependencies: [
    .package(url: "https://github.com/groue/GRDB.swift", from: "7.0.0"),
    .package(url: "https://github.com/apollographql/apollo-ios", exact: "2.4.0"),
    .package(path: "../PlatriumGraphQL"),
    .package(path: "../../sdk/_ffi/darwin"),
  ],
  targets: [
    .target(
      name: "PlatriumCore",
      dependencies: [
        .product(name: "GRDB", package: "GRDB.swift"),
        .product(name: "Apollo", package: "apollo-ios"),
        .product(name: "PlatriumGraphQL", package: "PlatriumGraphQL"),
        .product(name: "PlatriumSDK", package: "darwin"),
      ]
    ),
    .testTarget(
      name: "PlatriumCoreTests",
      dependencies: [
        "PlatriumCore",
        .product(name: "Apollo", package: "apollo-ios"),
        .product(name: "PlatriumGraphQL", package: "PlatriumGraphQL"),
        .product(name: "PlatriumSDK", package: "darwin"),
      ]
    ),
  ],
  swiftLanguageModes: [.v5]
)
