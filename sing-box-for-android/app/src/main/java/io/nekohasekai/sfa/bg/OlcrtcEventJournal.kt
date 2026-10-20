package io.nekohasekai.sfa.bg

import android.content.Context
import io.nekohasekai.sfa.BuildConfig
import java.io.File
import java.security.MessageDigest
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

// Keep diagnostic timestamps available on the app's minSdk without java.time desugaring.
internal fun olcrtcUtcNow(): String =
    SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'", Locale.US).apply {
        timeZone = TimeZone.getTimeZone("UTC")
    }.format(Date())

// ai-generated: bounded private OLCRTC lifecycle journal with deterministic redaction.
class OlcrtcEventJournal(
    private val directory: File,
    private val maxBytes: Long = 256 * 1024,
    private val generations: Int = 4,
) {
    private val lock = Any()

    fun append(raw: String) {
        if (!isCausalEvent(raw)) return
        val line = "utc=${olcrtcUtcNow()} ${redact(raw).replace('\n', ' ').replace('\r', ' ')}\n"
        synchronized(lock) {
            directory.mkdirs()
            val active = File(directory, ACTIVE_NAME)
            if (active.length() + line.toByteArray().size > maxBytes) rotate()
            active.appendText(line)
        }
    }

    private fun rotate() {
        File(directory, "$ACTIVE_NAME.${generations - 1}").delete()
        for (index in generations - 2 downTo 1) {
            val source = File(directory, "$ACTIVE_NAME.$index")
            val target = File(directory, "$ACTIVE_NAME.${index + 1}")
            if (source.exists()) {
                target.delete()
                source.renameTo(target)
            }
        }
        val active = File(directory, ACTIVE_NAME)
        if (active.exists()) {
            val first = File(directory, "$ACTIVE_NAME.1")
            first.delete()
            active.renameTo(first)
        }
        // ai-generated: enforce the configured bound even after an interrupted or failed rename.
        val allowed = buildSet {
            add(ACTIVE_NAME)
            for (index in 1 until generations) add("$ACTIVE_NAME.$index")
        }
        directory.listFiles()?.filter { it.name.startsWith("$ACTIVE_NAME.") && it.name !in allowed }?.forEach(File::delete)
    }

    private fun redact(raw: String): String {
        var value = URL.replace(raw, "<redacted-url>")
        value = SECRET.replace(value) { "${it.groupValues[1]}=<redacted>" }
        value = IDENTIFIER.replace(value) { "${it.groupValues[1]}=${safeHash(it.groupValues[2])}" }
        return value.take(MAX_LINE_CHARS)
    }

    private fun safeHash(value: String): String =
        MessageDigest.getInstance("SHA-256").digest(value.toByteArray()).take(6).joinToString("") { "%02x".format(it) }

    private fun isCausalEvent(message: String): Boolean = CAUSAL_MARKERS.any(message::contains)

    companion object {
        const val ACTIVE_NAME = "olcrtc-events.log"
        private const val MAX_LINE_CHARS = 4096
        private val URL = Regex("(?:https?|wss?)://\\S+", RegexOption.IGNORE_CASE)
        private val SECRET = Regex("(?i)\\b(token|access_token|refresh_token|cookie|cookies|psk|key|room_url|join_link|url|authorization)=\\S+")
        private val IDENTIFIER = Regex("(?i)\\b(device|session|sid|participant|track)=([A-Za-z0-9._:-]+)")
        private val CAUSAL_MARKERS = listOf(
            "OLCRTC outbound carrier", "connector generation=", "livekit diagnostic", "DisconnectReason",
            "vp8channel: readVP8Track closed", "vp8channel: local transport close", "vp8channel: reader progress",
            "ice_state", "candidate_pair", "signaling", "websocket close", "server ping recovery grace",
            "rotation received", "rotation applied", "rotation ignored", "rotation received-ack", "rotation ready",
            "rotation ack queued", "context ended", "client control stream ended", "service stop", "service close",
            "OLCRTC-Network",
            "call:", "call[wbstream]", "wbstream", "[lk]", "[lk-video]", "relay:",
        )
    }
}

// ai-generated: process-wide entry point backed by Android private storage.
object OlcrtcEventJournalStore {
    @Volatile private var journal: OlcrtcEventJournal? = null

    fun append(context: Context, event: String) {
        if (!BuildConfig.DEBUG) return
        val current = journal ?: synchronized(this) {
            journal ?: OlcrtcEventJournal(File(context.filesDir, "olcrtc-events")).also { journal = it }
        }
        current.append(event)
    }
}
