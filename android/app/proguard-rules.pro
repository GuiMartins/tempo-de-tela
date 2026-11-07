# Retrofit builds its service implementations from generic signatures at
# runtime, so R8 must not strip them.
-keepattributes Signature, InnerClasses, EnclosingMethod
-keepattributes RuntimeVisibleAnnotations, RuntimeVisibleParameterAnnotations
-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keep,allowobfuscation,allowshrinking class kotlin.coroutines.Continuation

# OkHttp references these optional platform classes only on JVMs that have them.
-dontwarn okhttp3.internal.platform.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**

# kotlinx.serialization generates a companion .serializer() looked up
# reflectively; without this the DTOs fail to deserialize in release only.
-keepclassmembers class com.tempodetela.** {
    *** Companion;
}
-keepclasseswithmembers class com.tempodetela.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# Tink (the crypto engine behind EncryptedSharedPreferences) references
# compile-time-only annotations.
-dontwarn com.google.errorprone.annotations.**
-dontwarn javax.annotation.**
