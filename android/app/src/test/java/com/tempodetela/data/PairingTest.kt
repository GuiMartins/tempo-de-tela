package com.tempodetela.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class PairingTest {
    @Test
    fun parsesPanelQr() {
        val cfg = Pairing.parseQr(
            """{"v":1,"urls":["http://192.168.31.172:8994","http://100.97.73.4:8994/","http://192.168.31.172:8994"],"token":"tdt_ABC"}""",
        )
        assertEquals(listOf("http://192.168.31.172:8994", "http://100.97.73.4:8994"), cfg.urls)
        assertEquals("tdt_ABC", cfg.token)
    }

    @Test
    fun rejectsOtherQrCodes() {
        assertThrows(IllegalArgumentException::class.java) { Pairing.parseQr("https://example.com") }
        assertThrows(IllegalArgumentException::class.java) { Pairing.parseQr("""{"urls":["http://a:1"],"token":"x"}""") }
    }

    @Test
    fun manualAddressGetsDefaultPort() {
        assertEquals("http://192.168.0.10:8994", Pairing.manual("192.168.0.10", " tdt_x ").urls.single())
        assertEquals("http://zima.local:9000", Pairing.normalizeUrl("zima.local:9000/"))
        assertEquals("http://[fd00::1]:8994", Pairing.normalizeUrl("[fd00::1]"))
        assertThrows(IllegalArgumentException::class.java) { Pairing.manual("192.168.0.10", "abc") }
    }

    @Test
    fun preferredUrlComesFirst() {
        val cfg = ServerConfig(listOf("http://a:1", "http://b:2"), "tdt_x", preferred = "http://b:2")
        assertEquals(listOf("http://b:2", "http://a:1"), cfg.orderedUrls())
    }
}
