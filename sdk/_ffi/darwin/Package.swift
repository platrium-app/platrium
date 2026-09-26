// swift-tools-version:5.5
import PackageDescription

let package = Package(
    name: "PlatriumSDK",
    platforms: [
        .iOS(.v16),
        .macOS(.v11)
    ],
    products: [
        .library(
            name: "PlatriumSDK",
            targets: ["PlatriumSDK"]
        ),
    ],
    targets: [
        .target(
            name: "PlatriumSDK",
            dependencies: ["PlatriumSDKCore"],
            path: "Sources/PlatriumSDK"
        ),
        .binaryTarget(
            name: "PlatriumSDKCore",
            path: "PlatriumSDKCore.xcframework"
        )
    ]
)
