UPDATE customers
SET
    contact_person = CASE
        WHEN COALESCE(contact_person, '') = '' THEN 'Калмыков Александр Федорович'
        ELSE contact_person
    END,
    contact_position = CASE
        WHEN COALESCE(contact_position, '') = '' THEN 'Директор'
        ELSE contact_position
    END
WHERE inn = '5257120323'
  AND (
      COALESCE(contact_person, '') = ''
      OR COALESCE(contact_position, '') = ''
  );
