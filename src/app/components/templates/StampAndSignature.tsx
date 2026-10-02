interface StampProps {
  visible: boolean
}

// Размер и смещения подобраны по образцу реального подписанного акта (фото
// от руки поставленных печати+подписи) — крупнее печатного блока, печать
// перекрывает строку подписи снизу, подпись перекрывает строку и ФИО сверху.

export function Stamp({ visible }: StampProps) {
  if (!visible) return null
  return (
    <img
      data-stamp
      src="/signature/stamp.png"
      alt=""
      className="absolute pointer-events-none"
      style={{ width: '190px', height: 'auto', left: '0px', top: '-34px', zIndex: 1 }}
    />
  )
}

export function Signature({ visible }: StampProps) {
  if (!visible) return null
  return (
    <img
      data-stamp
      src="/signature/signature.png"
      alt=""
      className="absolute pointer-events-none"
      style={{ width: '230px', height: 'auto', left: '0px', top: '-38px', transform: 'rotate(-6deg)', zIndex: 2 }}
    />
  )
}
