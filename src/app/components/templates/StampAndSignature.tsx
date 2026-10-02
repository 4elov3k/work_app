interface StampProps {
  visible: boolean
}

export function Stamp({ visible }: StampProps) {
  if (!visible) return null
  return (
    <img
      data-stamp
      src="/signature/stamp.png"
      alt=""
      className="absolute"
      style={{ width: '110px', height: 'auto', left: '10px', top: '-20px' }}
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
      style={{ width: '130px', height: 'auto' }}
    />
  )
}
