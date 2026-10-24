package io.nekohasekai.sfa.compose.screen.simple

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class CallTransportSignalTest {
    @Test fun recoveryThenConnection() {
        assertEquals(CallTransportSignal.Recovering, CallTransportSignal.fromLog("outbound/call[vk]: vk-joiner: tunnel lost"))
        assertEquals(CallTransportSignal.Recovering, CallTransportSignal.fromLog("telemost-joiner: reconnect attempt #3"))
        assertEquals(CallTransportSignal.Connected, CallTransportSignal.fromLog("vk-joiner: === TUNNEL CONNECTED ==="))
        assertEquals(CallTransportSignal.Connected, CallTransportSignal.fromLog("telemost-joiner: === VP8 TUNNEL CONNECTED ==="))
    }

    @Test fun terminalTransportFailures() {
        assertEquals(CallTransportSignal.Failed, CallTransportSignal.fromLog("outbound/call[vk]: call: joiner connection failed"))
        assertEquals(CallTransportSignal.Failed, CallTransportSignal.fromLog("vk-joiner: gave up after 5 consecutive reconnect attempts"))
    }

    @Test fun destinationErrorsAndRetryWarningsAreNotTerminalFailures() {
        assertNull(CallTransportSignal.fromLog("connection: connection refused"))
        assertNull(CallTransportSignal.fromLog("telemost-joiner: timeout, will retry"))
        assertNull(CallTransportSignal.fromLog("vk-joiner: PC disconnected, closing transport to trigger reconnect"))
        assertNull(CallTransportSignal.fromLog("ordinary outbound: TUNNEL CONNECTED"))
    }
}
