package io.nekohasekai.sfa.bg

import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.lifecycle.lifecycleScope
import io.nekohasekai.sfa.R
import kotlinx.coroutines.launch

/** Open only a currently pending, core-issued loopback challenge after a user tap. */
class VkCaptchaActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val link = intent.dataString
        if (link == null || VkCaptchaLink.port(link) == null || VkCaptchaAccess.activeLink != link) {
            showError()
            return
        }
        lifecycleScope.launch {
            val opened = runCatching {
                val view = Intent(Intent.ACTION_VIEW, Uri.parse(link)).addCategory(Intent.CATEGORY_BROWSABLE)
                // Chrome is the configured CAPTCHA browser; use the default if absent.
                val chrome = Intent(view).setPackage("com.android.chrome")
                if (packageManager.resolveActivity(chrome, PackageManager.MATCH_DEFAULT_ONLY) != null) {
                    view.setPackage("com.android.chrome")
                }
                val browser = packageManager.resolveActivity(view, PackageManager.MATCH_DEFAULT_ONLY)
                    ?.activityInfo ?: error("no browser")
                // A resolver dialog has no browser UID: ask the user to select a default browser.
                check(browser.packageName != "android" && browser.packageName != packageName)
                check(VkCaptchaAccess.activeLink == link)
                view.setClassName(browser.packageName, browser.name)
                startActivity(view)
            }.isSuccess
            if (!opened) showError() else finish()
        }
    }

    private fun showError() {
        Toast.makeText(this, R.string.vk_captcha_open_error, Toast.LENGTH_LONG).show()
        finish()
    }
}
