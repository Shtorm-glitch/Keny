package io.nekohasekai.sfa.utils

import org.junit.Assert.assertEquals
import org.junit.Test

class LegacyProfilePathsTest {
    @Test
    fun movesInternalProfileStorageToNewPackage() {
        assertEquals("/data/user/0/keny.sfa/files/configs/11.json", remapLegacyProfilePath("/data/user/0/io.nekohasekai.sfa/files/configs/11.json", "/data/user/0/keny.sfa/files", null))
    }

    @Test
    fun movesExternalWorkingFilesToNewPackage() {
        assertEquals("/storage/emulated/0/Android/data/keny.sfa/files/merged.srs", remapLegacyProfilePath("/sdcard/Android/data/io.nekohasekai.sfa/files/merged.srs", "/data/user/0/keny.sfa/files", "/storage/emulated/0/Android/data/keny.sfa/files"))
    }

    @Test
    fun preservesRemoteUrlsAndAlreadyMigratedPaths() {
        val remote = "https://example.invalid/io.nekohasekai.sfa/profile"
        assertEquals(remote, remapLegacyProfilePath(remote, "/new/files", "/new/external"))
        assertEquals("/new/files/configs/11.json", remapLegacyProfilePath("/new/files/configs/11.json", "/new/files", null))
    }
}
