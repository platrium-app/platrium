package org.platrium.platrium

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import org.platrium.platrium.ui.PlatriumApp
import org.platrium.platrium.ui.theme.PlatriumTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        deliverAuthCallback(intent)
        setContent {
            PlatriumTheme {
                PlatriumApp()
            }
        }
    }

    // singleTask: the server's redirect to platrium://auth/callback arrives here.
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        deliverAuthCallback(intent)
    }

    private fun deliverAuthCallback(intent: Intent?) {
        val data = intent?.data ?: return
        if (data.scheme == "platrium") {
            (application as PlatriumApplication).container.authenticator.deliver(data.toString())
        }
    }
}
