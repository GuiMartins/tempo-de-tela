package com.tempodetela.data

import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.Assert.assertEquals
import org.junit.Test
import java.net.ServerSocket

class FailoverTest {
    @Test
    fun fallsBackToNextAddressAndRemembersIt() = runBlocking {
        // Porta livre sem ninguém escutando: conexão recusada na hora.
        val dead = ServerSocket(0).use { "http://127.0.0.1:${it.localPort}" }
        val server = MockWebServer()
        server.enqueue(MockResponse().setBody("""{"version":"teste","dns":{"queries":3}}"""))
        server.start()
        val alive = server.url("/").toString().trimEnd('/')

        var preferred: String? = null
        val cfg = ServerConfig(listOf(dead, alive), "tdt_token")
        val status = buildApi(config = { cfg }, onWorking = { preferred = it }).status()

        assertEquals("teste", status.version)
        assertEquals(3, status.dns.queries)
        assertEquals(alive, preferred)
        val request = server.takeRequest()
        assertEquals("/api/v1/status", request.path)
        assertEquals("Bearer tdt_token", request.getHeader("Authorization"))
        server.shutdown()
    }
}
