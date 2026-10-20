package io.nekohasekai.sfa.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class KenyUpdatePolicyTest {
    @Test
    fun prefersSupportedArchitectureOverUniversal() {
        assertEquals(
            "keny.android-arm64-v8a.apk",
            KenyUpdatePolicy.selectApk(
                listOf("keny.android-universal.apk", "keny.android-x86_64.apk", "keny.android-arm64-v8a.apk"),
                listOf("arm64-v8a", "armeabi-v7a"),
                false,
            ),
        )
    }

    @Test
    fun rejectsOtherApplicationsAndUnsupportedArchitecture() {
        assertNull(KenyUpdatePolicy.selectApk(listOf("SFA-arm64-v8a.apk", "Keny-1.1-android-arm64-v8a.apk", "keny.android-x86_64.apk"), listOf("arm64-v8a"), false))
    }

    @Test
    fun keepsLegacySeparateAndAllowsUniversalFallback() {
        val names = listOf("keny.android-universal.apk", "keny.android-legacy-android-5-arm64-v8a.apk")
        assertEquals(names[0], KenyUpdatePolicy.selectApk(names, listOf("arm64-v8a"), false))
        assertEquals(names[1], KenyUpdatePolicy.selectApk(names, listOf("arm64-v8a"), true))
    }
}
