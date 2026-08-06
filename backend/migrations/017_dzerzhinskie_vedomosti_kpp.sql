UPDATE customers
SET
    kpp = '524901001',
    fullname = CASE
        WHEN fullname = '' OR fullname = name THEN 'Муниципальное автономное учреждение «Информационный центр «Дзержинские ведомости»'
        ELSE fullname
    END,
    address = CASE
        WHEN COALESCE(address, '') = '' THEN '606000, Нижегородская обл., г. Дзержинск, пр. Дзержинского, д. 9'
        ELSE address
    END,
    contact_person = CASE
        WHEN COALESCE(contact_person, '') = '' THEN 'Трескин Петр Андреевич'
        ELSE contact_person
    END,
    contact_position = CASE
        WHEN COALESCE(contact_position, '') = '' THEN 'Директор'
        ELSE contact_position
    END,
    phone = CASE
        WHEN COALESCE(phone, '') = '' THEN '8(8313) 27-99-79'
        ELSE phone
    END,
    email = CASE
        WHEN COALESCE(email, '') = '' THEN 'dzved@mail.ru'
        ELSE email
    END
WHERE inn = '5249091492'
  AND (
      COALESCE(kpp, '') = ''
      OR fullname = ''
      OR fullname = name
      OR COALESCE(address, '') = ''
      OR COALESCE(contact_person, '') = ''
      OR COALESCE(contact_position, '') = ''
      OR COALESCE(phone, '') = ''
      OR COALESCE(email, '') = ''
  );
