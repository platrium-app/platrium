package org.platrium.platrium.core.auth

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** Where bearer tokens live. They never go in the database. */
interface TokenVault {
    fun token(accountId: String): String?
    fun setToken(accountId: String, token: String)
    fun deleteToken(accountId: String)
}

/**
 * Tokens encrypted with an AES-GCM key held in the Android Keystore (hardware
 * backed where available, never exportable); only the ciphertext is stored.
 */
class KeystoreTokenVault(context: Context) : TokenVault {
    private val prefs = context.getSharedPreferences("platrium_tokens", Context.MODE_PRIVATE)

    override fun token(accountId: String): String? {
        val stored = prefs.getString(accountId, null) ?: return null
        return try {
            val bytes = Base64.decode(stored, Base64.NO_WRAP)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, IV_LENGTH))
            String(cipher.doFinal(bytes, IV_LENGTH, bytes.size - IV_LENGTH), Charsets.UTF_8)
        } catch (_: Exception) {
            // A key that can't open the blob (restored backup, wiped keystore)
            // is the same as having no token: the account needs signing in.
            null
        }
    }

    override fun setToken(accountId: String, token: String) {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val sealed = cipher.iv + cipher.doFinal(token.toByteArray(Charsets.UTF_8))
        check(prefs.edit().putString(accountId, Base64.encodeToString(sealed, Base64.NO_WRAP)).commit()) {
            "Couldn't store the token."
        }
    }

    override fun deleteToken(accountId: String) {
        prefs.edit().remove(accountId).apply()
    }

    private fun key(): SecretKey {
        val store = KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        (store.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, ANDROID_KEYSTORE).run {
            init(
                KeyGenParameterSpec.Builder(KEY_ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(256)
                    .build(),
            )
            generateKey()
        }
    }

    private companion object {
        const val ANDROID_KEYSTORE = "AndroidKeyStore"
        const val KEY_ALIAS = "org.platrium.tokens"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_LENGTH = 12
    }
}

class InMemoryTokenVault : TokenVault {
    private val tokens = java.util.concurrent.ConcurrentHashMap<String, String>()
    override fun token(accountId: String): String? = tokens[accountId]
    override fun setToken(accountId: String, token: String) { tokens[accountId] = token }
    override fun deleteToken(accountId: String) { tokens.remove(accountId) }
}
