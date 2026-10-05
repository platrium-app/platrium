package org.platrium.platrium.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test
import org.platrium.platrium.core.account.ServerUrl
import org.platrium.platrium.core.account.ServerUrlException

class ServerUrlTest {
    @Test
    fun publicHostsDefaultToHttps() {
        assertEquals("https://files.example.com", ServerUrl.normalize("files.example.com"))
        assertEquals("https://files.example.com", ServerUrl.normalize("  Files.Example.com/ "))
    }

    @Test
    fun localHostsDefaultToHttp() {
        assertEquals("http://localhost:3000", ServerUrl.normalize("localhost:3000"))
        assertEquals("http://192.168.1.20:3000", ServerUrl.normalize("192.168.1.20:3000"))
        assertEquals("http://10.0.2.2:3000", ServerUrl.normalize("10.0.2.2:3000"))
        assertEquals("http://nas.local", ServerUrl.normalize("nas.local"))
    }

    @Test
    fun keepsExplicitSchemeAndPathButDropsQueryAndTrailingSlash() {
        assertEquals("http://example.com/platrium", ServerUrl.normalize("http://example.com/platrium/?x=1#y"))
    }

    @Test
    fun rejectsBadInput() {
        assertThrows(ServerUrlException.Empty::class.java) { ServerUrl.normalize("   ") }
        assertThrows(ServerUrlException.UnsupportedScheme::class.java) { ServerUrl.normalize("ftp://example.com") }
        assertThrows(ServerUrlException.Invalid::class.java) { ServerUrl.normalize("https://user:pw@example.com") }
    }
}
