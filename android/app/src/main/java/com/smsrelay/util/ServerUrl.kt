package com.smsrelay.util

object ServerUrl {
    fun normalize(raw: String): String {
        var url = raw.trim()
        if (url.endsWith("/")) {
            url = url.removeSuffix("/")
        }
        if (url.endsWith("/api/v1")) {
            url = url.removeSuffix("/api/v1")
        }
        return url
    }
}
