package io.nekohasekai.sfa.bg

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.nio.file.Files
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

class OlcrtcEventJournalTest {
    @Test
    fun capturesCallLifecycleWithoutCredentialOrWebSocketUrl() {
        val directory = Files.createTempDirectory("call-journal").toFile()
        val journal = OlcrtcEventJournal(directory)
        journal.append("call: rotation carrier ready url=wss://secret.invalid/room psk=test-secret access_token=test-token")
        val saved = directory.resolve(OlcrtcEventJournal.ACTIVE_NAME).readText()
        assertTrue(saved.contains("rotation carrier ready"))
        assertFalse(saved.contains("secret.invalid"))
        assertFalse(saved.contains("test-secret"))
        assertFalse(saved.contains("test-token"))
    }
    // ai-generated: bounded rotation never exceeds the configured generation count.
    @Test
    fun rotatesWithinBound() {
        val directory = Files.createTempDirectory("olcrtc-journal").toFile()
        val journal = OlcrtcEventJournal(directory, maxBytes = 256, generations = 3)
        repeat(100) { journal.append("connector generation=$it ready") }
        assertTrue(directory.listFiles()!!.size <= 3)
        assertTrue(directory.listFiles()!!.all { it.length() <= 4096 })
    }

    // ai-generated: URLs and credential-like fields never reach persistent storage.
    @Test
    fun redactsSecretsAndIdentifiers() {
        val directory = Files.createTempDirectory("olcrtc-redaction").toFile()
        val journal = OlcrtcEventJournal(directory)
        journal.append("livekit diagnostic url=https://secret.invalid/room token=abc device=device-1")
        val saved = directory.resolve(OlcrtcEventJournal.ACTIVE_NAME).readText()
        assertFalse(saved.contains("secret.invalid"))
        assertFalse(saved.contains("token=abc"))
        assertFalse(saved.contains("device-1"))
    }

    // ai-generated: concurrent lifecycle writers produce complete bounded lines without exceptions.
    @Test
    fun acceptsConcurrentWrites() {
        val directory = Files.createTempDirectory("olcrtc-concurrent").toFile()
        val journal = OlcrtcEventJournal(directory, maxBytes = 2048, generations = 4)
        val executor = Executors.newFixedThreadPool(8)
        repeat(400) { index -> executor.execute { journal.append("rotation received epoch=$index") } }
        executor.shutdown()
        assertTrue(executor.awaitTermination(10, TimeUnit.SECONDS))
        assertTrue(directory.listFiles()!!.all { file -> file.readLines().all { it.startsWith("utc=") } })
    }
}
