package io.nekohasekai.sfa.bg

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class VkCaptchaLinkTest {
    @Test fun acceptsOnlyBareLoopbackChallenge() {
        assertEquals(4567, VkCaptchaLink.port("http://127.0.0.1:4567/"))
        listOf("https://127.0.0.1:4567/", "http://evil.test:4567/",
            "http://127.0.0.1:0/", "http://127.0.0.1:65536/",
            "http://user@127.0.0.1:4567/", "http://127.0.0.1:4567/?token=x",
            "http://127.0.0.1:4567/path", "http://127.0.0.1:4567/#x")
            .forEach { assertNull(it, VkCaptchaLink.port(it)) }
    }
}
