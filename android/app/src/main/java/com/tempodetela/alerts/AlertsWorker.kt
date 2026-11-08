package com.tempodetela.alerts

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.tempodetela.MainActivity
import com.tempodetela.R
import com.tempodetela.TempoApp
import java.util.concurrent.TimeUnit

/**
 * A cada 15 min (o mínimo do WorkManager), pede ao servidor os alertas novos e mostra uma
 * notificação para cada um. Sem Firebase: nada sai da casa.
 */
class AlertsWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val app = applicationContext as TempoApp
        if (app.serverStore.config.value == null) return Result.success()
        val prefs = applicationContext.getSharedPreferences("alertas", Context.MODE_PRIVATE)
        val lastSeen = prefs.getLong("ultimo_id", -1)
        val events = try {
            app.api.events(after = if (lastSeen < 0) null else lastSeen, limit = 50)
        } catch (e: Exception) {
            return Result.retry()
        }
        if (events.isEmpty()) {
            // Servidor ainda sem alertas: os próximos que surgirem já são novidade.
            if (lastSeen < 0) prefs.edit().putLong("ultimo_id", 0).apply()
            return Result.success()
        }
        prefs.edit().putLong("ultimo_id", events.maxOf { it.id }).apply()
        // Primeira vez: só marca até onde já viu, sem despejar o histórico em notificações.
        if (lastSeen < 0) return Result.success()
        // Com ntfy, o servidor já entregou na hora: notificar aqui de novo seria em dobro.
        val ntfy = runCatching { app.api.status().ntfyUrl }.getOrDefault("")
        if (ntfy.isNotBlank()) return Result.success()

        val manager = NotificationManagerCompat.from(applicationContext)
        if (!manager.areNotificationsEnabled()) return Result.success()
        ensureChannel(applicationContext)
        val open = PendingIntent.getActivity(
            applicationContext, 0,
            Intent(applicationContext, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        events.takeLast(5).forEach { e ->
            val notification = NotificationCompat.Builder(applicationContext, CHANNEL)
                .setSmallIcon(R.drawable.ic_notification)
                .setContentTitle(e.title)
                .setContentText(e.message)
                .setStyle(NotificationCompat.BigTextStyle().bigText(e.message))
                .setContentIntent(open)
                .setAutoCancel(true)
                .setWhen(e.ts * 1000)
                .setShowWhen(true)
                .build()
            try {
                manager.notify(e.id.toInt(), notification)
            } catch (_: SecurityException) {
                // Permissão de notificação revogada entre a checagem e o envio.
            }
        }
        return Result.success()
    }

    companion object {
        const val CHANNEL = "alertas"
        private const val WORK = "alertas-periodicos"

        fun ensureChannel(context: Context) {
            val manager = context.getSystemService(NotificationManager::class.java)
            if (manager.getNotificationChannel(CHANNEL) == null) {
                manager.createNotificationChannel(
                    NotificationChannel(CHANNEL, "Alertas", NotificationManager.IMPORTANCE_DEFAULT).apply {
                        description = "Avisos de uso, limite estourado, uso na hora de dormir ou na aula, app novo, aparelho novo na rede"
                    },
                )
            }
        }

        /** Roda a verificação uma vez agora (usado pela build de debug para testar). */
        fun runNow(context: Context) {
            WorkManager.getInstance(context).enqueue(OneTimeWorkRequestBuilder<AlertsWorker>().build())
        }

        fun schedule(context: Context) {
            val request = PeriodicWorkRequestBuilder<AlertsWorker>(15, TimeUnit.MINUTES)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .build()
            WorkManager.getInstance(context).enqueueUniquePeriodicWork(WORK, ExistingPeriodicWorkPolicy.KEEP, request)
        }
    }
}
