// swift-tools-version:5.7
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
            dependencies: ["platrium_sdkFFI"],
            path: "Sources/PlatriumSDK"
        ),
        .binaryTarget(
            name: "platrium_sdkFFI",
            path: "PlatriumSDKFFI.xcframework"
        )
    ]
)
