/// Initializes cross-platform logging for the SDK.
/// This should only be called once when the SDK is instantiated.
pub fn init_xplat_logging() {
    #[cfg(target_os = "android")]
    {
        android_logger::init_once(
            android_logger::Config::default()
                .with_max_level(log::LevelFilter::Debug)
                .with_tag("PlatriumSDK"),
        );
    }

    // For other platforms (iOS, Web, Native), they can either use their own loggers
    // or standard stdout. We don't need to do anything here right now.
}
