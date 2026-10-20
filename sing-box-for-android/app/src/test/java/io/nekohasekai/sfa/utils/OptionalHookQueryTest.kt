package io.nekohasekai.sfa.utils

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class OptionalHookQueryTest {
    @Test
    fun deniedOldModuleDoesNotCrashNewPackage() {
        assertNull(optionalHookQuery<String> { throw SecurityException("unauthorized") })
    }

    @Test
    fun successfulQueryKeepsItsResult() {
        assertEquals("active", optionalHookQuery { "active" })
    }

    @Test(expected = IllegalArgumentException::class)
    fun programmingErrorsAreNotHidden() {
        optionalHookQuery<String> { throw IllegalArgumentException("invalid query") }
    }
}
