package com.tempodetela

import android.app.Application
import com.tempodetela.data.Api
import com.tempodetela.data.ServerStore
import com.tempodetela.data.buildApi

class TempoApp : Application() {
    lateinit var serverStore: ServerStore
        private set
    lateinit var api: Api
        private set

    override fun onCreate() {
        super.onCreate()
        serverStore = ServerStore(this)
        api = buildApi(config = { serverStore.config.value }, onWorking = serverStore::setPreferred)
    }
}
