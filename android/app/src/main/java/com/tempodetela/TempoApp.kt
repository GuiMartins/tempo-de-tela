package com.tempodetela

import android.app.Application
import com.tempodetela.data.Api
import com.tempodetela.data.ServerStore
import com.tempodetela.data.buildApi
import com.tempodetela.update.Updater
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

class TempoApp : Application() {
    lateinit var serverStore: ServerStore
        private set
    lateinit var api: Api
        private set
    lateinit var updater: Updater
        private set

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onCreate() {
        super.onCreate()
        serverStore = ServerStore(this)
        api = buildApi(config = { serverStore.config.value }, onWorking = serverStore::setPreferred)
        updater = Updater(this)
        scope.launch { updater.check() }
    }
}
