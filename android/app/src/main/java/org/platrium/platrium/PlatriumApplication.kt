package org.platrium.platrium

import android.app.Application
import android.os.Build
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import org.platrium.platrium.core.account.AccountRepository
import org.platrium.platrium.core.account.AccountStatus
import org.platrium.platrium.core.account.AccountStore
import org.platrium.platrium.core.account.AppDatabase
import org.platrium.platrium.core.auth.CustomTabAuthenticator
import org.platrium.platrium.core.auth.DeviceDescriptor
import org.platrium.platrium.core.auth.KeystoreTokenVault
import org.platrium.platrium.core.auth.SignInService
import org.platrium.platrium.core.network.ClientFactory
import org.platrium.platrium.core.transfer.DownloadManager

class PlatriumApplication : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
    }
}

/** The app's long-lived objects, built once per process. */
class AppContainer(app: Application) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    val repository = AccountRepository(AppDatabase.open(app).accounts())
    private val vault = KeystoreTokenVault(app)

    val downloads = DownloadManager(app, scope).also { it.clearCache() }
    val authenticator = CustomTabAuthenticator(app)

    val device = DeviceDescriptor(
        name = Build.MODEL,
        appVersion = BuildConfig.VERSION_NAME,
    )

    private val clients = ClientFactory(
        vault = vault,
        // The server turned the token down (revoked or expired): ask the user to sign in again.
        onUnauthorized = { accountId -> scope.launch { repository.setStatus(accountId, AccountStatus.NEEDS_REAUTH) } },
        onSdkCreated = downloads::attach,
    )

    val accounts = AccountStore(
        repository = repository,
        clients = clients,
        signIn = SignInService(repository, vault, device),
        authenticator = authenticator,
        scope = scope,
    )
}
