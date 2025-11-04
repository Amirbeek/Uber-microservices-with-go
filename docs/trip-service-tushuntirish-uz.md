# Trip-service muammolarini tushuntirish (Uzbek)

Bu hujjat frontendga tegmasdan, backend (trip-service va API Gateway) tarafida qilingan o‘zgarishlar, nima uchun qilinganligi va tekshirish tartibini sodda qilib tushuntiradi.

## 1) Umumiy arxitektura
- Frontend → API Gateway (HTTP, 8081) → Trip-service (gRPC, 9093) → OSRM (tashqi marshrut servisi)
- Kubernetes’da `trip-service` degan Service nomi orqali API Gateway gRPC bilan ulanadi.

## 2) Ko‘rilgan alomatlar
- 502 Bad Gateway (API Gateway trip-service bilan ulanganida xato)
- gRPC handler ichida `log.Fatal` sababli trip-service konteyneri to‘satdan yopilib qolishi
- `Cannot use 'estimatedFares' ... as []*RideFareModel` (interfeys signaturalari mos emas)
- Frontend’da `icon` xatosi (packageSlug noto‘g‘ri bo‘lgani uchun meta topilmaydi)
- Xarita chizig‘i ko‘rinmasligi (koordinatalar tartibi front kutayotgan formatga mos emas)

## 3) Qilingan tuzatishlar va “nima uchun”

### 3.1. Portlar va ulanish manzili
- Trip-service gRPC serveri default portini `:9093` ga birxillashtirdik.
- API Gateway gRPC mijozida default manzilni `trip-service:9093` qildik.
  - Nima uchun: Kubernetes ichida servis nomi + port orqali sog‘lom ulanish ta’minlanadi.

### 3.2. Trip-service gRPC handler’da xatolarni qayta ishlash
- `log.Fatalf` o‘rniga `log.Printf` va gRPC status qaytarildi.
  - Nima uchun: `log.Fatalf` processni to‘xtatadi. Bu konteynerni yiqitadi va 502 ga olib keladi.

### 3.3. Interfeys signaturalari mosligi
- `TripService` interfeysida:
  - `EstimatePackagesPriceWithRoute` endi `[]*RideFareModel` qaytaradi (bitta emas, bir nechta paket baholari).
  - `GenerateTripFares` ga `route` ham uzatiladi va `[]*RideFareModel` qaytaradi.
  - Nima uchun: hisob-kitob bosqichi kolleksiya bilan ishlaydi; oldin bitta tip qaytargan joyda massiv talab qilindi — type xatosi shundan.

### 3.4. Noto‘liq/keraksiz funksiya olib tashlandi
- API Gateway’da yarim qolgan `CreatTrip()` funksiya olib tashlandi.
  - Nima uchun: build xatolarini keltirib chiqarayotgan edi.

### 3.5. Paket slugdagi xatolik
- `"vam"` yozuvi `"van"` ga tuzatildi.
  - Nima uchun: Frontend `PackagesMeta` lug‘atida `van` mavjud, `vam` bo‘lsa `icon` topilmaydi va xato keladi.

### 3.6. Koordinatalar tartibi (eng nozik joy)
- OSRM koordinatalarni `[lon, lat]` qaytaradi.
- Frontend esa serverdan kelgan obyektni `coord.longitude, coord.latitude` tarzida `[lon, lat]` qilib polyline’ga beradi.
- Shuning uchun backendda `pb.Coordinate` maydonlarini “muvofiqlashtirish” kerak:
  - `Latitude` ga OSRM’dagi `lon` (coord[0])
  - `Longitude` ga OSRM’dagi `lat` (coord[1])
- Nima uchun: Frontend kodi o‘zgarmaydi, lekin u `[longitude, latitude]` ni chiziq chizish uchun ishlatadi. Shu “swap” frontning mavjud mantiqiga mos tushadi va chiziq to‘g‘ri chiziladi.

## 4) Endi ma’lumot oqimi qanday ishlaydi
1) Frontend `/trip/preview` ga so‘rov yuboradi.
2) API Gateway gRPC orqali `trip-service:9093` ga ulanadi.
3) Trip-service OSRM’dan marshrut oladi, koordinatalarni front kutilayotgan tartibga moslab yuboradi.
4) Paketlar bo‘yicha taxminiy narxlar (sedan, suv, van, luxury) hisoblanadi va qaytariladi.

## 5) Tekshirish tartibi (quick checklist)
- Kubernetes:
  - `trip-service` Service porti: 9093
  - `api-gateway` HTTP: 8081
- Muhit o‘zgaruvchilari:
  - API Gateway: `GATEWAY_HTTP_ADDR=:8081`, `TRIP_SERVICE_URL=trip-service:9093` (agar kerak bo‘lsa)
  - Trip-service: `GRPC_ADDR=:9093`
- So‘rov/JavaScript xatolari:
  - `icon` xatosi ketgan bo‘lishi kerak (`van` slug to‘g‘ri)
  - 502 xatolari yo‘q (trip-service yiqilmayapti)

## 6) Frontend kutayotgan JSON shakli (soddalashtirilgan)
```
{
  "route": {
    "geometry": [
      { "coordinates": [ {"latitude": <...>, "longitude": <...>}, ... ] }
    ],
    "distance": <number>,
    "duration": <number>
  },
  "rideFares": [
    {"id": "...", "userID": "...", "packageSlug": "sedan|suv|van|luxury", "totalPriceInCents": <number>},
    ...
  ]
}
```
- Eslatma: koordinatalar obyekt ko‘rinishida keladi; frontend ularni `[coord.longitude, coord.latitude]` ga aylantirib polyline chizadi.

## 7) Nega bular qilindi — qisqa xulosa
- Portlar va servis manzillari birxillashtirildi → tarmoq muammolari bartaraf etildi.
- Handler’lar to‘g‘ri xatoni qaytaradi → servis yiqilmaydi.
- Interfeys imzolari to‘g‘rilandi → type xatolari yo‘qoldi.
- `van` slug tuzatildi → frontend ikonka va karta ro‘yxatini to‘g‘ri ko‘rsatadi.
- Koordinata “swap” saqlandi → mavjud frontend mantiqi bilan chiziq to‘g‘ri chiziladi.
