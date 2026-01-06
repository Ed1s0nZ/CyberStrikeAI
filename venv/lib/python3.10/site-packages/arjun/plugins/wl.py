def detect_casing(string):
    """Detect the casing style and delimiter of given string."""
    delimiter = ""
    casing = ""

    if string.islower():
        casing = "l"
    elif string.isupper():
        casing = "u"
    else:
        casing = casing = "c" if string[0].islower() else "p"

    if "-" in string:
        delimiter = "-"
    elif "_" in string:
        delimiter = "_"
    elif "." in string:
        delimiter = "."

    return delimiter, casing


def transform(parts, delimiter, casing):
    """Combine list of strings to form a string with given casing style."""
    if len(parts) == 1:
        if casing == "l":
            return parts[0].lower()
        elif casing == "u":
            return parts[0].upper()
        return parts[0]

    result = []
    for i, part in enumerate(parts):
        if casing == "l":
            transformed = part.lower()
        elif casing == "u":
            transformed = part.upper()
        elif casing == "c":
            if i == 0:
                transformed = part.lower()
            else:
                transformed = part.lower().title()
        else:  # casing == "p"
            transformed = part.lower().title()

        result.append(transformed)

    return delimiter.join(result)


def handle(text):
    """Break down a string into array of 'words'."""
    if "-" in text:
        return text.split("-")
    elif "_" in text:
        return text.split("_")
    elif "." in text:
        return text.split(".")

    if not text.islower() and not text.isupper():
        parts = []
        temp = ""
        for char in text:
            if not char.isupper():
                temp += char
            else:
                if temp:
                    parts.append(temp)
                temp = char
        if temp:
            parts.append(temp)
        return parts

    return [text]


def covert_to_case(string, delimiter, casing):
    """Process input stream and write transformed text to output stream."""
    parts = handle(string)
    return transform(parts, delimiter, casing)
