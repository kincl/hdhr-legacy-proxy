
def format_msg(msg, as_bytes):
    if as_bytes:
        return bytes(f"{msg}\n", "utf-8")
    else:
        return msg
