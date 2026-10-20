package io.nekohasekai.sfa.bg

import java.net.URI

object VkCaptchaLink {
    const val CHANNEL = "vk-captcha"
    const val NOTIFICATION_ID = 24014

    fun port(link: String): Int? = runCatching {
        val uri = URI(link)
        uri.port.takeIf {
            uri.scheme == "http" && uri.host == "127.0.0.1" && it in 1..65535 &&
                uri.rawUserInfo == null && uri.rawQuery == null && uri.rawFragment == null &&
                uri.rawPath == "/"
        }
    }.getOrNull()
}
