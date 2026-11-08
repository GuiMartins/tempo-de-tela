package com.tempodetela.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ReleasesTest {
    @Test
    fun parsesLatestRelease() {
        val update = GitHubReleases.parse(
            """{"tag_name":"v0.1.2","name":"v0.1.2","assets":[
              {"name":"notes.txt","browser_download_url":"https://x/notes.txt","size":10},
              {"name":"tempo-de-tela-v0.1.2.apk","browser_download_url":"https://x/app.apk","size":2100000}]}""",
        )
        assertEquals(AppUpdate("0.1.2", "https://x/app.apk", 2_100_000), update)
        assertNull(GitHubReleases.parse("""{"tag_name":"v0.1.3","assets":[]}"""))
    }

    @Test
    fun comparesVersionsNumberByNumber() {
        assertTrue(isNewer("0.10.0", "0.9.1"))
        assertTrue(isNewer("0.1.2", "0.1.1"))
        assertTrue(isNewer("1.0", "0.9.9"))
        assertFalse(isNewer("0.1.1", "0.1.1"))
        assertFalse(isNewer("0.1.0", "0.1.1"))
        assertFalse(isNewer("nightly", "0.1.1"))
        // Build local "0.0.0-dev" é mais velho que qualquer release.
        assertTrue(isNewer("0.1.1", "0.0.0-dev"))
    }
}
