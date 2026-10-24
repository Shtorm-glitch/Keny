package io.nekohasekai.sfa.compose.screen.simple

/** Only transport lifecycle messages, never arbitrary failed destination requests. */
enum class CallTransportSignal {
    Connected, Recovering, Failed;

    companion object {
        fun fromLog(message: String): CallTransportSignal? = when {
            message.contains("call: joiner connection failed") -> Failed
            message.contains("vk-joiner: gave up after ") -> Failed
            (message.contains("vk-joiner:") || message.contains("telemost-joiner:")) &&
                message.contains("TUNNEL CONNECTED") -> Connected
            (message.contains("vk-joiner:") || message.contains("telemost-joiner:")) &&
                (message.contains("tunnel lost") || message.contains("reconnect attempt #") ||
                    message.contains("forcing reconnect:")) -> Recovering
            else -> null
        }
    }
}
