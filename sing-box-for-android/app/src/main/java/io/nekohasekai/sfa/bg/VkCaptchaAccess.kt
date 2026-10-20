package io.nekohasekai.sfa.bg

/** Only core-issued, pending loopback links may be opened from notifications. */
object VkCaptchaAccess {
    @Volatile var activeLink: String? = null
        private set

    fun activate(link: String) {
        activeLink = link
    }

    fun clear() {
        activeLink = null
    }
}
