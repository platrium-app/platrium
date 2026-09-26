#!/usr/bin/env bash
set -e

# Change to the sdk root directory
cd "$(dirname "$0")/.."

echo "Building for macOS (Apple Silicon)..."
MACOSX_DEPLOYMENT_TARGET=11.0 cargo build --release --target aarch64-apple-darwin

echo "Building for macOS (Intel)..."
MACOSX_DEPLOYMENT_TARGET=11.0 cargo build --release --target x86_64-apple-darwin

echo "Building for iOS Device (ARM64)..."
IPHONEOS_DEPLOYMENT_TARGET=16.0 cargo build --release --target aarch64-apple-ios

echo "Building for iOS Simulator (ARM64)..."
IPHONEOS_DEPLOYMENT_TARGET=16.0 cargo build --release --target aarch64-apple-ios-sim

echo "Building for iOS Simulator (Intel)..."
IPHONEOS_DEPLOYMENT_TARGET=16.0 cargo build --release --target x86_64-apple-ios

# Create working directories for the build
echo "Setting up workspace..."
mkdir -p _ffi/darwin/Sources/PlatriumSDK
mkdir -p _ffi/darwin/build/lipo/macos _ffi/darwin/build/lipo/ios_sim _ffi/darwin/build/lipo/ios
mkdir -p _ffi/darwin/build/bindings
mkdir -p _ffi/darwin/build/headers

# Generate bindings (using the macOS aarch64 library)
echo "Generating Swift bindings via UniFFI..."
cargo run --manifest-path uniffi/Cargo.toml -- generate \
    --library target/aarch64-apple-darwin/release/libplatrium_sdk.a \
    --config uniffi/uniffi.toml \
    --language swift \
    --out-dir _ffi/darwin/build/bindings

# Move the Swift file to the SPM Sources directory
echo "Copying Swift files..."
cp _ffi/darwin/build/bindings/*.swift _ffi/darwin/Sources/PlatriumSDK/

# Move headers and modulemap to a central location for the xcframework
echo "Copying C Headers..."
cp _ffi/darwin/build/bindings/*.h _ffi/darwin/build/bindings/*.modulemap _ffi/darwin/build/headers/

# Lipo macOS binaries
echo "Creating Universal macOS library..."
lipo -create -output _ffi/darwin/build/lipo/macos/libplatrium_sdk.a \
    target/aarch64-apple-darwin/release/libplatrium_sdk.a \
    target/x86_64-apple-darwin/release/libplatrium_sdk.a

# Lipo iOS simulator binaries
echo "Creating Universal iOS Simulator library..."
lipo -create -output _ffi/darwin/build/lipo/ios_sim/libplatrium_sdk.a \
    target/aarch64-apple-ios-sim/release/libplatrium_sdk.a \
    target/x86_64-apple-ios/release/libplatrium_sdk.a

# Copy iOS device binary
echo "Copying iOS Device library..."
cp target/aarch64-apple-ios/release/libplatrium_sdk.a _ffi/darwin/build/lipo/ios/libplatrium_sdk.a

# Remove old xcframework if it exists
echo "Cleaning old XCFramework..."
rm -rf _ffi/darwin/PlatriumSDK.xcframework

# Create XCFramework
echo "Creating XCFramework..."
xcodebuild -create-xcframework \
    -library _ffi/darwin/build/lipo/macos/libplatrium_sdk.a -headers _ffi/darwin/build/headers \
    -library _ffi/darwin/build/lipo/ios_sim/libplatrium_sdk.a -headers _ffi/darwin/build/headers \
    -library _ffi/darwin/build/lipo/ios/libplatrium_sdk.a -headers _ffi/darwin/build/headers \
    -output _ffi/darwin/PlatriumSDK.xcframework

# Clean up intermediate build artifacts
echo "Cleaning up intermediate build files..."
rm -rf _ffi/darwin/build

echo "Successfully built Darwin framework at _ffi/darwin/PlatriumSDK.xcframework"
