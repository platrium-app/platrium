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

    #[cfg(target_arch = "wasm32")]
    {
        console_log::init_with_level(log::Level::Debug).unwrap_or(());
    }

    #[cfg(all(
        not(target_arch = "wasm32"),
        not(target_os = "android"),
        not(target_os = "ios")
    ))]
    {
        env_logger::Builder::from_env(env_logger::Env::default().filter("PLATRIUM_SDK_LOGLEVEL"))
            .init();
    }
}
