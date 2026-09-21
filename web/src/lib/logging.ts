import log from 'loglevel';

/**
 * Creates a tagged logger with debug level enabled by default.
 */
export function createLogger(tag: string): log.Logger {
    const logger = log.getLogger(tag);

    if (!(logger as any)._configured) {
        logger.setLevel("debug"); // Enable debug logs

        // Inject prefix into logger
        const originalFactory = logger.methodFactory;
        logger.methodFactory = function (methodName, logLevel, loggerName) {
            const rawMethod = originalFactory(methodName, logLevel, loggerName);
            return function (...args: any[]) {
                rawMethod(`[${String(loggerName)}]`, ...args);
            };
        };

        // Re-apply the log level to bind the new method factory
        logger.setLevel(logger.getLevel());
        (logger as any)._configured = true;
    }

    return logger;
}
