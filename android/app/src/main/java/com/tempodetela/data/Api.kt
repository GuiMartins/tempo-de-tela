package com.tempodetela.data

import kotlinx.serialization.json.JsonObject
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface Api {
    @GET("api/v1/status")
    suspend fun status(): ServerStatus

    @GET("api/v1/devices")
    suspend fun devices(@Query("filter") filter: String = "all"): List<Device>

    @PATCH("api/v1/devices/{id}")
    suspend fun patchDevice(@Path("id") id: Long, @Body body: JsonObject): Device

    /** Corpo {"person_id": N} ou {"person_id": null}. */
    @PUT("api/v1/devices/{id}/person")
    suspend fun assignDevice(@Path("id") id: Long, @Body body: JsonObject): Device

    @GET("api/v1/persons")
    suspend fun persons(): List<Person>

    @POST("api/v1/persons")
    suspend fun createPerson(@Body body: JsonObject): Person

    @PATCH("api/v1/persons/{id}")
    suspend fun patchPerson(@Path("id") id: Long, @Body body: JsonObject): Person

    @DELETE("api/v1/persons/{id}")
    suspend fun deletePerson(@Path("id") id: Long): Response<Unit>
}
